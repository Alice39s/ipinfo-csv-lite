#!/usr/bin/env python3
import gzip
import lzma
import shutil
from pathlib import Path

def compress_file():
    input_file = Path("libs/ipinfo-lite.csv")
    
    # Create gzip
    with input_file.open('rb') as f_in:
        with gzip.open(f"{input_file}.gz", 'wb') as f_out:
            shutil.copyfileobj(f_in, f_out)
    
    # Create xz
    with input_file.open('rb') as f_in:
        with lzma.open(f"{input_file}.xz", 'wb') as f_out:
            shutil.copyfileobj(f_in, f_out)

if __name__ == "__main__":
    compress_file() 