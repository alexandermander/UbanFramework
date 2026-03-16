# UBANFRAMEWORK (UEFI binray analysis Framework)

This project provides a **Docker-based framework for analyzing UEFI firmware images** and automatically extracting and decompiling PE modules.

The framework is designed for firmware research and reverse engineering and focuses on firmware images such as:

* `.fd`
* `.img`
* `.iso`

The pipeline extracts PE modules from the firmware image and decompiles them using **Ghidra Headless**.

---

# Tools Used

This project combines several tools:

* **Ghidra** – reverse engineering framework
* **ghidra-firmware-utils** – extension for analyzing UEFI firmware structures
* **UEFIExtract** – extracts firmware components from UEFI images
* **Bash + Python scripts** – automation pipeline

---

# Pipeline Overview

The framework performs the following steps:

1. Extract firmware components using **UEFIExtract**
2. Identify PE modules inside the firmware image
3. Import the PE modules into **Ghidra Headless**
4. Run helper scripts for UEFI analysis
5. Export the decompiled C code

---

# Installation

The project is designed to run inside Docker to ensure a reproducible environment.

Build the Docker image:

```bash
docker build -t ubanframework .
```

This will install:

* Java (JDK)
* Ghidra
* ghidra-firmware-utils
* UEFIExtract
* required Python tools

---

# Running the Framework

The output directory must be synchronized with your host machine.

## Linux

```bash
docker run --rm -it \
-v $(pwd)/out_decomplied_files:/app/out_decomplied_files \
ubanframework
```

This mounts the container output directory to your local project directory.

All generated files will appear in:

```
./out_decomplied_files
```

---

## Windows (PowerShell)

```powershell
docker run --rm -it -v ${PWD}/out_decomplied_files:/app/out_decomplied_files ubanframework
```

---

# Output

All generated files will be written to:

```
out_decomplied_files/
```

For each detected PE module the framework creates:

```
out_decomplied_files/
 ├── ModuleName/
 │   ├── main.efi
 │   └── decompiled_main.c
```

* `main.efi` – extracted PE module
* `decompiled_main.c` – Ghidra decompiled output

---

# Processing Limit

Decompiling firmware modules using Ghidra can take significant time.

For testing purposes the script currently processes **only the first 5 PE modules**.

This limit is controlled in:

```
run_decompile.sh
```

```bash
MAX=5
```

You can increase this value depending on your analysis needs.

---

# Project Structure

```
.
├── Dockerfile
├── run_decompile.sh
├── run_decompile_parallel.py
├── get_PE_files.sh
├── ExportDecompiled.java
├── test_VM_firmware.fd
├── out_decomplied_files/
└── README.md
```

---

# Notes

* The framework is intended for **UEFI firmware analysis and reverse engineering**.
* The Docker container ensures that all dependencies are correctly installed.
* The scripts can easily be extended to process larger firmware datasets.

---

# Future Improvements

Possible improvements include:

* parallel decompilation of firmware modules
* automatic GUID identification
* integration with vulnerability detection pipelines
* automated firmware dataset processing
