import csv
from itertools import islice
from concurrent.futures import ThreadPoolExecutor


def process_asn(asn):
    """Process ASN, remove 'AS' prefix, return 0 if empty"""
    if not asn or not asn.strip():
        return 0
    try:
        return int(str(asn).replace("AS", "").strip())
    except ValueError:
        return 0


def process_chunk(chunk):
    """Process a chunk of rows from the new IPinfo format

    New format columns:
    0: network (CIDR), 1: country, 2: country_code, 3: continent,
    4: continent_code, 5: asn, 6: as_name, 7: as_domain

    Output format columns:
    cidr, country_code, continent_code, as_number, as_name
    """
    results = []
    for row in chunk:
        # Skip malformed rows
        if len(row) < 7:
            continue

        network = row[0].strip()
        if not network:
            continue

        results.append(
            [
                network,  # CIDR (already in CIDR format)
                row[2],  # country_code
                row[4],  # continent_code
                process_asn(row[5]),  # as_number
                row[6] or "",  # as_name
            ]
        )
    return results


def process_csv(input_file, output_file, chunk_size=5000):
    """Process the IPinfo CSV file and extract relevant fields"""
    with open(input_file, "r") as f_in, open(output_file, "w", newline="") as f_out:
        reader = csv.reader(f_in)
        next(reader)  # skip header
        writer = csv.writer(f_out)
        writer.writerow(
            ["cidr", "country_code", "continent_code", "as_number", "as_name"]
        )

        with ThreadPoolExecutor(max_workers=8) as executor:
            while True:
                chunk = list(islice(reader, chunk_size))
                if not chunk:
                    break

                future = executor.submit(process_chunk, chunk)
                results = future.result()
                writer.writerows(results)


def main():
    process_csv("libs/country_asn.csv", "libs/ipinfo-lite.csv")


if __name__ == "__main__":
    main()
