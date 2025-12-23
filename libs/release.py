import gzip
import lzma
import shutil
from pathlib import Path


def compress_file():
    """Compress the ipinfo-lite.csv file to both gzip and xz formats"""
    input_file = Path("libs/ipinfo-lite.csv")

    if not input_file.exists():
        print(f"Error: {input_file} not found")
        return

    # Create gzip
    print(f"Creating {input_file}.gz...")
    with input_file.open("rb") as f_in:
        with gzip.open(f"{input_file}.gz", "wb") as f_out:
            shutil.copyfileobj(f_in, f_out)

    # Create xz
    print(f"Creating {input_file}.xz...")
    with input_file.open("rb") as f_in:
        with lzma.open(f"{input_file}.xz", "wb") as f_out:
            shutil.copyfileobj(f_in, f_out)

    print("Compression completed.")


def main():
    compress_file()


if __name__ == "__main__":
    main()
