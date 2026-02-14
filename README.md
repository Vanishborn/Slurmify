# Slurmify

**Slurmify** is a lightweight, high-performance CLI tool written in Go that automates the creation of Slurm batch scripts (`.sbatch`) from a simple list of commands. It is designed for high-throughput computing, automatically handling job naming, log directory management, and command formatting.

## Features

* **Batch Generation:** Converts a text file of commands into individual `.sbatch` files efficiently.
* **Smart Naming:** Automatically derives meaningful job names from input files or output flags.
* **Safe Defaults:** Creates organized output (`./Sbatch`) and log (`./Logs`) directories with secure permissions.
* **Pretty Printing:** Formats complex, multi-line commands with proper line continuation (`\`) for readability.
* **Shell Safety:** Correctly handles quoting for arguments containing spaces and preserves shell operators (`>`, `|`, `>>`).

## Installation

### Pre-compiled Binary

Download the latest release from the [Releases Page](https://github.com/Vanishborn/Slurmify/releases).

### Build from Source

Go 1.25+ required.

Clone the repo and use Make to build the versioned binary:

```zsh
git clone https://github.com/Vanishborn/slurmify.git
cd slurmify
make
```

## Usage

Prepare a text file (e.g., `commands.txt`) containing one command per line:

```zsh
bwa mem -t 16 ref.fa sample1.fq > sample1.sam
samtools sort -o sample1.sorted.bam sample1.sam
fastqc --outdir ./QC sample2.fastq.gz
```

Run `slurmify`:

```zsh
./slurmify -I commands.txt -A my_account
```

### Generated Output

The tool will create a `./Sbatch` directory containing scripts like `sample1.sbatch`.

**Example Content (`sample1.sbatch`):**

```bash
#!/bin/bash
#SBATCH --job-name=sample1
#SBATCH --account=my_account
#SBATCH --partition=standard
#SBATCH --nodes=1
#SBATCH --ntasks=1
#SBATCH --cpus-per-task=1
#SBATCH --mem=4G
#SBATCH --time=01:00:00
#SBATCH --output=./Logs/sample1_%j.out
#SBATCH --error=./Logs/sample1_%j.err

set -euo pipefail
echo "[$(date)] Job $SLURM_JOB_ID running on $(hostname)"

# Command
bwa mem \
  -t 16 \
  ref.fa \
  sample1.fq \
  > sample1.sam
```

## Configuration Flags

|  Flag  | Description                              |  Default   | Required |
| :----: | ---------------------------------------- | :--------: | :------: |
| **-I** | Input text file with commands            |     -      | **Yes**  |
| **-A** | Slurm account name                       |     -      | **Yes**  |
| **-O** | Output directory for `.sbatch` files     | `./Sbatch` |    No    |
| **-L** | Directory for Slurm logs (`.out`/`.err`) |  `./Logs`  |    No    |
| **-P** | Slurm partition                          | `standard` |    No    |
| **-C** | CPUs per task                            |    `1`     |    No    |
| **-M** | Memory per task                          |    `4G`    |    No    |
| **-T** | Walltime (Format: HH:MM:SS)              | `01:00:00` |    No    |
| **-G** | GRES string                              |     -      |    No    |
| **-E** | Email for notifications                  |     -      |    No    |
| **-J** | Job name prefix                          |   `job`    |    No    |
| **-m** | Environment module to load               |     -      |    No    |

## Future Directions

* **Job Arrays:** Support for generating Slurm job arrays.
* **Template Customization:** Allow users to provide a custom user-defined template for the `.sbatch` header.
* **Dependency Handling:** Logic to chain jobs based on input order.
