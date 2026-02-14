package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/shlex"
)

var version = "dev"

// --- GLOBALS ---

// Pre-compile regex
var safeArgPattern = regexp.MustCompile(`^[a-zA-Z0-9_\-\./@=:,+]+$`)

// Extensions to strip
var trimExts = map[string]bool{
	".bam": true, ".sam": true, ".cram": true, ".bai": true, ".gz": true,
	".bed": true, ".bw": true, ".txt": true, ".sorted": true, ".csi": true,
	".tbi": true, ".fq": true, ".fastq": true, ".fa": true, ".fasta": true,
	".fai": true, ".vcf": true, ".csv": true, ".tsv": true, ".log": true,
	".out": true, ".err": true, ".json": true, ".yaml": true, ".yml": true,
}

// Config holds all Slurm job configuration parameters
type Config struct {
	InputFile string
	OutputDir string
	LogsDir   string
	Partition string
	Account   string
	Gres      string
	CPUs      int
	Mem       string
	Time      string
	Email     string
	JobPrefix string
	Module    string
}

// --- ENTRY POINT ---

func main() {
	// Allows defers to execute before os.Exit
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "[slurmify] Fatal Error: %v\n", err)
		os.Exit(1)
	}
}

// Run orchestrates the execution flow
func run() error {
	conf, err := parseFlags()
	if err != nil {
		return err
	}

	// Setup directories
	if err := os.MkdirAll(conf.OutputDir, 0755); err != nil {
		return fmt.Errorf("could not create output directory: %w", err)
	}
	if err := os.MkdirAll(conf.LogsDir, 0755); err != nil {
		return fmt.Errorf("could not create logs directory: %w", err)
	}

	// Process file
	count, err := processInputFile(conf)
	if err != nil {
		return err
	}

	fmt.Printf("[slurmify] Generated %d script(s) in %s/\n", count, conf.OutputDir)
	fmt.Printf("[slurmify] Logs destination in %s/\n", conf.LogsDir)
	return nil
}

// --- CORE LOGIC ---

func processInputFile(conf Config) (int, error) {
	file, err := os.Open(conf.InputFile)
	if err != nil {
		return 0, fmt.Errorf("could not open input file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	count := 0

	for scanner.Scan() {
		cmd := strings.TrimSpace(scanner.Text())

		if cmd == "" || strings.HasPrefix(cmd, "#") {
			continue
		}
		count++

		// Generate
		jobName := deriveJobName(cmd, conf.JobPrefix, count)
		scriptContent := generateScript(cmd, jobName, conf)

		// File Write
		filename := resolveFilename(conf.OutputDir, jobName, count)
		if err := os.WriteFile(filename, []byte(scriptContent), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "[slurmify] Warning: Could not write %s: %v\n", filename, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return count, fmt.Errorf("could not read input file: %w", err)
	}

	return count, nil
}

// resolveFilename handles collisions
func resolveFilename(dir, jobName string, index int) string {
	filename := filepath.Join(dir, fmt.Sprintf("%s.sbatch", jobName))
	// If file exists, append index
	if _, err := os.Stat(filename); err == nil {
		filename = filepath.Join(dir, fmt.Sprintf("%s_%03d.sbatch", jobName, index))
	}
	return filename
}

// generateScript builds the full content of the .sbatch file
func generateScript(cmd, jobName string, c Config) string {
	var sb strings.Builder

	// 1. Header
	writeSbatchHeader(&sb, jobName, c)

	// 2. Body Setup
	sb.WriteString("\nset -euo pipefail\n")
	sb.WriteString("echo \"[$(date)] Job $SLURM_JOB_ID running on $(hostname)\"\n")
	if c.Gres != "" {
		sb.WriteString("echo \"[$(date)] CUDA_VISIBLE_DEVICES=${CUDA_VISIBLE_DEVICES:-unset}\"\n")
	}
	sb.WriteString("\n")

	if c.Module != "" {
		sb.WriteString(fmt.Sprintf("module load %s\n\n", c.Module))
	}

	// 3. Command
	sb.WriteString("# Command\n")
	writePrettyCommand(&sb, cmd)

	return sb.String()
}

// writeSbatchHeader handles the #SBATCH lines
func writeSbatchHeader(sb *strings.Builder, jobName string, c Config) {
	sb.WriteString("#!/bin/bash\n")
	sb.WriteString(fmt.Sprintf("#SBATCH --job-name=%s\n", jobName))
	sb.WriteString(fmt.Sprintf("#SBATCH --account=%s\n", c.Account))
	sb.WriteString(fmt.Sprintf("#SBATCH --partition=%s\n", c.Partition))
	sb.WriteString("#SBATCH --nodes=1\n")
	sb.WriteString("#SBATCH --ntasks=1\n")
	sb.WriteString(fmt.Sprintf("#SBATCH --cpus-per-task=%d\n", c.CPUs))
	sb.WriteString(fmt.Sprintf("#SBATCH --mem=%s\n", c.Mem))
	sb.WriteString(fmt.Sprintf("#SBATCH --time=%s\n", c.Time))
	sb.WriteString(fmt.Sprintf("#SBATCH --output=%s/%s_%%j.out\n", c.LogsDir, jobName))
	sb.WriteString(fmt.Sprintf("#SBATCH --error=%s/%s_%%j.err\n", c.LogsDir, jobName))

	if c.Gres != "" {
		sb.WriteString(fmt.Sprintf("#SBATCH --gres=%s\n", c.Gres))
	}
	if c.Email != "" {
		sb.WriteString(fmt.Sprintf("#SBATCH --mail-user=%s\n", c.Email))
		sb.WriteString("#SBATCH --mail-type=BEGIN,END,FAIL\n")
	}
}

// writePrettyCommand handles the shlex splitting and line breaking
func writePrettyCommand(sb *strings.Builder, cmd string) {
	tokens, err := shlex.Split(cmd)
	if err != nil {
		// Fallback to raw string if quotes are unbalanced
		sb.WriteString(cmd + "\n")
		return
	}

	var lines []string
	i := 0
	for i < len(tokens) {
		token := tokens[i]
		var curr string

		if isShellOperator(token) {
			curr = token
		} else {
			curr = quoteArg(token)
		}

		// Check if this is a flag followed by a value
		if strings.HasPrefix(curr, "-") && i+1 < len(tokens) {
			next := tokens[i+1]
			if !strings.HasPrefix(next, "-") && !isShellOperator(next) {
				curr = fmt.Sprintf("%s %s", curr, quoteArg(next))
				i++
			}
		}
		lines = append(lines, curr)
		i++
	}

	sb.WriteString(strings.Join(lines, " \\\n  ") + "\n")
}

// --- HELPER FUNCTIONS ---

// deriveJobName extracted to keep main clean
func deriveJobName(cmd, prefix string, idx int) string {
	parts := strings.Fields(cmd)
	base := ""

	// Check for > redirect or -o flag
	for i, part := range parts {
		if (part == ">" || part == "-o" || part == "-O" || part == "--output") && i+1 < len(parts) {
			base = filepath.Base(parts[i+1])
			break
		}
	}

	// Fallback to last argument
	if base == "" && len(parts) > 0 {
		base = filepath.Base(parts[len(parts)-1])
	}

	if base != "" {
		// Strip extensions loop
		for {
			ext := filepath.Ext(base)
			if ext == "" || !trimExts[ext] {
				break
			}
			base = strings.TrimSuffix(base, ext)
		}
		// Sanitize
		base = strings.ReplaceAll(base, "*", "")
		base = strings.ReplaceAll(base, "?", "")
	}

	if base == "" {
		return fmt.Sprintf("%s_%04d", prefix, idx)
	}
	return fmt.Sprintf("%s_%s", prefix, base)
}

func quoteArg(s string) string {
	if s == "" {
		return "''"
	}
	if safeArgPattern.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// isShellOperator uses a switch for O(1)
func isShellOperator(s string) bool {
	switch s {
	case ">", ">>", "<", "|", "2>", "1>", "&>", "&&", "||", ";":
		return true
	}
	return false
}

func parseFlags() (Config, error) {
	c := Config{}
	flag.StringVar(&c.InputFile, "I", "", "Input text file with commands (Required)")
	flag.StringVar(&c.OutputDir, "O", "./Sbatch", "Output directory for .sbatch files")
	flag.StringVar(&c.LogsDir, "L", "./Logs", "Directory for Slurm logs")
	flag.StringVar(&c.Partition, "P", "standard", "Slurm partition")
	flag.StringVar(&c.Account, "A", "", "Slurm account (Required)")
	flag.StringVar(&c.Gres, "G", "", "GPU GRES string")
	flag.IntVar(&c.CPUs, "C", 1, "CPUs per task")
	flag.StringVar(&c.Mem, "M", "4G", "Memory per task")
	flag.StringVar(&c.Time, "T", "01:00:00", "Walltime")
	flag.StringVar(&c.Email, "E", "", "Email for notifications")
	flag.StringVar(&c.JobPrefix, "J", "job", "Job name prefix")
	flag.StringVar(&c.Module, "m", "", "Module to load")

	var showVersion bool
    flag.BoolVar(&showVersion, "V", false, "Show version and exit")

	flag.Parse()

	if showVersion {
		fmt.Printf("Slurmify %s\n", version)
		os.Exit(0)
	}

	if c.InputFile == "" || c.Account == "" {
		return c, fmt.Errorf("error: required flags -I (Input) and -A (Account) are missing")
	}
	return c, nil
}
