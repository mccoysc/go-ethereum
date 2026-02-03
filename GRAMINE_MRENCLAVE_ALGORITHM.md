# Gramine MRENCLAVE Calculation Algorithm

## Source Analysis

Based on study of `/tmp/gramine/python/graminelibos/sgx_sign.py`

## Core Algorithm

```python
mrenclave = hashlib.sha256()

# Step 1: ECREATE
data = struct.pack('<8sLQ44s', b'ECREATE', SSA_FRAME_SIZE // PAGESIZE, enclave_size, b'')
mrenclave.update(data)

# Step 2: For each memory area (in order)
for area in memory_areas:
    for page_addr in range(area.addr, area.addr + area.size, PAGESIZE):
        # EADD
        offset = page_addr - enclave_base
        data = struct.pack('<8sQQ40s', b'EADD', offset, flags, b'')
        mrenclave.update(data)
        
        # EEXTEND (if measured)
        if area.measure:
            for chunk_offset in range(0, PAGESIZE, 256):
                data = struct.pack('<8sQ48s', b'EEXTEND', offset + chunk_offset, b'')
                mrenclave.update(data)
                mrenclave.update(page_content[chunk_offset:chunk_offset+256])

return mrenclave.digest()  # 32 bytes
```

## Constants

```python
PAGESIZE = 4096  # 1 << 12
SSA_FRAME_SIZE = PAGESIZE * 4  # 16384
ENCLAVE_STACK_SIZE = PAGESIZE * 64  # 262144
ENCLAVE_SIG_STACK_SIZE = PAGESIZE * 16  # 65536
TCS_SIZE = 4096  # sizeof(sgx_arch_tcs_t)
```

## Memory Areas (in order)

1. **Manifest** - The manifest data itself
   - Size: length of manifest TOML
   - Flags: PAGEINFO_R | PAGEINFO_REG
   - Measured: Yes
   
2. **SSA** (Save State Area)
   - Size: max_threads * SSA_FRAME_SIZE * 2
   - Flags: PAGEINFO_R | PAGEINFO_W | PAGEINFO_REG  
   - Measured: No

3. **TCS** (Thread Control Structure)
   - Size: max_threads * TCS_SIZE
   - Flags: PAGEINFO_TCS
   - Measured: No

4. **TLS** (Thread Local Storage)
   - Size: max_threads * PAGESIZE
   - Flags: PAGEINFO_R | PAGEINFO_W | PAGEINFO_REG
   - Measured: No

5. **Stack**
   - Size: max_threads * ENCLAVE_STACK_SIZE
   - Flags: PAGEINFO_R | PAGEINFO_W | PAGEINFO_REG
   - Measured: No

6. **Signal Stack**
   - Size: max_threads * ENCLAVE_SIG_STACK_SIZE
   - Flags: PAGEINFO_R | PAGEINFO_W | PAGEINFO_REG
   - Measured: No

7. **PAL** (libpal.so)
   - Size: determined by ELF file
   - Flags: varies by section (R/W/X permissions)
   - Measured: Yes
   - Note: Must load and parse actual ELF file

8. **Free Areas**
   - Fill gaps between allocated areas
   - Measured: No

## Page Flags

```python
PAGEINFO_R = 0x1    # Read
PAGEINFO_W = 0x2    # Write  
PAGEINFO_X = 0x4    # Execute
PAGEINFO_TCS = 0x100
PAGEINFO_REG = 0x200
```

## struct.pack Format Strings

- **ECREATE**: `'<8sLQ44s'`
  - 8 bytes: "ECREATE" string
  - 4 bytes: SSA_FRAME_SIZE / PAGESIZE (uint32)
  - 8 bytes: enclave_size (uint64)
  - 44 bytes: padding (zeros)
  - Total: 64 bytes

- **EADD**: `'<8sQQ40s'`
  - 8 bytes: "EADD" string
  - 8 bytes: offset (uint64)
  - 8 bytes: flags (uint64)
  - 40 bytes: padding (zeros)
  - Total: 64 bytes

- **EEXTEND**: `'<8sQ48s'` + content
  - 8 bytes: "EEXTEND" string
  - 8 bytes: offset (uint64)
  - 48 bytes: padding (zeros)
  - 256 bytes: page content chunk
  - Total: 320 bytes

## Key Insights

1. **Order Matters**: Memory areas must be processed in exact order
2. **Alignment**: All addresses must be page-aligned (4096 bytes)
3. **libpal Required**: Must load and process actual libpal.so ELF file
4. **Manifest Content**: The manifest TOML bytes are included as first area
5. **Not All Pages Measured**: Only manifest and libpal pages are measured (EEXTEND)
6. **Deterministic**: Same manifest + libpal = same MRENCLAVE

## Why Previous Implementation Failed

1. **Oversimplified**: Didn't include all memory areas
2. **Wrong Order**: Didn't follow Gramine's exact ordering
3. **Missing libpal**: Didn't load and process PAL binary
4. **Incomplete Areas**: Only processed manifest, missed SSA/TCS/TLS/Stack/etc
5. **Wrong Formats**: Didn't use exact struct pack formats

## Correct Implementation Requirements

To implement correctly in Go:

1. Read manifest file and parse SGX config (enclave_size, max_threads)
2. Load libpal.so and parse as ELF file
3. Create memory areas in exact order
4. Calculate enclave_base and assign addresses to areas
5. Process ECREATE with correct format
6. For each area, process each page with EADD
7. For measured pages (manifest, libpal), process EEXTEND for each 256-byte chunk
8. Return SHA256 digest

## Test Data

From our test manifest (test.manifest.sgx):
- Expected MRENCLAVE: `faa284c4d200890541c4515810ef8ad2065c18a4c979cfb1e16ee5576fe014ee`
- Enclave Size: 2GB (0x80000000)
- Max Threads: 32
- Debug: true

## References

- Gramine source: `python/graminelibos/sgx_sign.py`
- Constants: `pal/src/host/linux-sgx/pal_linux_defs.h`
- SGX arch: `pal/src/host/linux-sgx/sgx_arch.h`
