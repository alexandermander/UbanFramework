FROM eclipse-temurin:21-jdk-jammy

WORKDIR /app

RUN apt-get update && apt-get install -y \
    python3 \
    python3-pip \
    file \
    wget \
    unzip \
    && rm -rf /var/lib/apt/lists/*

# install ghidra
RUN wget https://github.com/NationalSecurityAgency/ghidra/releases/download/Ghidra_12.0.4_build/ghidra_12.0.4_PUBLIC_20260303.zip \
    -O /tmp/ghidra.zip && \
    unzip /tmp/ghidra.zip -d /opt && \
    rm /tmp/ghidra.zip

RUN wget https://github.com/al3xtjames/ghidra-firmware-utils/releases/download/2026.01.14/ghidra_12.0_PUBLIC_20260114_ghidra-firmware-utils.zip \
    -O /tmp/firmware-utils.zip && \
    unzip /tmp/firmware-utils.zip -d /opt/ghidra_12.0.4_PUBLIC/Ghidra/Extensions && \
    rm /tmp/firmware-utils.zip

RUN wget https://github.com/LongSoft/UEFITool/releases/download/A73/UEFIExtract_NE_A73_x64_linux.zip \
    -O /tmp/UEFIExtract_NE_A73_x64_linux.zip && \
    unzip /tmp/UEFIExtract_NE_A73_x64_linux.zip -d /opt/ && \
    rm /tmp/UEFIExtract_NE_A73_x64_linux.zip

RUN mv /opt/uefiextract /usr/bin/
ENV GHIDRA_HOME=/opt/ghidra_12.0.4_PUBLIC
ENV PATH="$GHIDRA_HOME/support:/opt/UEFIExtract:$PATH"

COPY run_decompile.sh .
COPY get_PE_files.sh .
COPY run_decompile_parallel.py .
COPY ExportDecompiled.java .
COPY test_VM_firmware.fd .

RUN uefiextract test_VM_firmware.fd all

RUN chmod +x get_PE_files.sh
RUN ./get_PE_files.sh

RUN chmod +x ./run_decompile.sh

CMD ["./run_decompile.sh"]

