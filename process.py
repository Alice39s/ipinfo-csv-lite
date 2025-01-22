import csv
import ipaddress
from itertools import islice
from concurrent.futures import ThreadPoolExecutor


def process_asn(asn):
    """Process ASN, remove 'AS' prefix, return 0 if empty"""
    if not asn:
        return 0
    return int(str(asn).replace("AS", "").strip())


def ip_to_cidr(start_ip, end_ip):
    try:
        if ":" in start_ip:  # IPv6
            start = ipaddress.IPv6Address(start_ip)
            end = ipaddress.IPv6Address(end_ip)
        else:  # IPv4
            start = ipaddress.IPv4Address(start_ip)
            end = ipaddress.IPv4Address(end_ip)
        return str(next(ipaddress.summarize_address_range(start, end)))
    except:
        return None


def process_chunk(chunk):
    results = []
    for row in chunk:
        cidr = ip_to_cidr(row[0], row[1])
        if cidr:
            # Reserved: CIDR, country_code, continent_code, as_number, as_name
            results.append(
                [
                    cidr,  # CIDR
                    row[2],  # country_code
                    row[4],  # continent_code
                    process_asn(row[6]),  # as_number
                    row[7] or "",  # as_name
                ]
            )
    return results


def process_csv(input_file, output_file, chunk_size=5000):
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


if __name__ == "__main__":
    process_csv("libs/country_asn.csv", "libs/ipinfo-lite.csv")
