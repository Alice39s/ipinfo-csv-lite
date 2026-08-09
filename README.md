# ipinfo-csv-lite

A lightweight, multi-format distribution of the IPinfo Lite country and ASN
database, updated daily at 12:00 UTC.

# Download

| Format       | Latest Release Download Link                                                                |
| ------------ | ------------------------------------------------------------------------------------------- |
| CSV          | `https://github.com/alice39s/ipinfo-csv-lite/releases/latest/download/ipinfo-lite.csv`      |
| CSV (Gzip)   | `https://github.com/alice39s/ipinfo-csv-lite/releases/latest/download/ipinfo-lite.csv.gz`   |
| CSV (Lzma)   | `https://github.com/alice39s/ipinfo-csv-lite/releases/latest/download/ipinfo-lite.csv.xz`   |
| CSV (Zstd)   | `https://github.com/alice39s/ipinfo-csv-lite/releases/latest/download/ipinfo-lite.csv.zst`  |
| MMDB         | `https://github.com/alice39s/ipinfo-csv-lite/releases/latest/download/ipinfo-lite.mmdb`     |
| XDB (IPv4)   | `https://github.com/alice39s/ipinfo-csv-lite/releases/latest/download/ipinfo-lite.ipv4.xdb` |
| XDB (IPv6)   | `https://github.com/alice39s/ipinfo-csv-lite/releases/latest/download/ipinfo-lite.ipv6.xdb` |
| SHA-256 Sums | `https://github.com/alice39s/ipinfo-csv-lite/releases/latest/download/checksums.txt`        |

All releases are published as [immutable releases](https://docs.github.com/en/code-security/supply-chain-security/understanding-your-software-supply-chain/immutable-releases) — assets and tags cannot be modified after publication. Every artifact also carries a signed build-provenance attestation binding it to the source commit, verifiable with:

```bash
gh attestation verify ipinfo-lite.mmdb --repo Alice39s/ipinfo-csv-lite
```

You can also download the latest release from the [Releases](https://github.com/Alice39s/ipinfo-csv-lite/releases/latest) page.

# Data Structure

The generated CSV uses the reduced five-field schema below. XDB stores the
same values in its compact region string. The MMDB asset intentionally keeps
IPinfo's official schema instead; see [MMDB Record](#mmdb-record).

## Python

```python
from typing import TypedDict

class IPInfo(TypedDict):
    cidr: str
    country_code: str
    continent_code: str
    as_number: int
    as_name: str
```

## Go

```go
type IPInfo struct {
    CIDR          string  `json:"cidr"`
    CountryCode   string  `json:"country_code"`
    ContinentCode string  `json:"continent_code"`
    ASNumber      int     `json:"as_number"`
    ASName        string `json:"as_name"`
}
```

## TypeScript

```typescript
interface IPInfo {
  cidr: string;
  country_code: string;
  continent_code: string;
  as_number: number;
  as_name: string;
}
```

## MMDB Record

The `.mmdb` release asset is an unchanged, byte-for-byte mirror of IPinfo's
official `ipinfo_lite.mmdb`, after SHA-256 and MMDB-format validation. It is not
rebuilt from the reduced CSV, so it retains the complete official record
schema and metadata and remains directly compatible with existing IPinfo Lite
MMDB consumers.

```json
{
  "asn": "AS13335",
  "as_name": "Cloudflare, Inc.",
  "as_domain": "cloudflare.com",
  "country": "Australia",
  "country_code": "AU",
  "continent": "Oceania",
  "continent_code": "OC"
}
```

## XDB Region

The `.xdb` files follow the [ip2region xdb v3 format](https://github.com/lionsoul2014/ip2region), usable with any xdb searcher client. Each segment's region string is:

```
country_code|continent_code|as_number|as_name
```

## Requirements

- Go 1.24+ (only needed to build the pipeline; using published release assets requires nothing)
- Optional: `xz` from xz-utils enables the faster parallel LZMA encoder; a pure-Go fallback is built in

## Usage

### Automatic (requires an IPinfo token)

```bash
export IPINFO_TOKEN=<your-token>
make
```

This runs `update`, which downloads the official CSV and MMDB and verifies each
against IPinfo's SHA-256 endpoint before extracting the CSV. It then builds the
reduced CSV, compressed archives and XDB files while publishing the MMDB
unchanged, and finally writes release checksums. Outputs land in `./data/`.
Individual steps remain available as `make update`, `make process`, etc.; checks
via `make vet test`.

### Manual (no token)

1. Download both the CSV and MMDB versions of "IPinfo Lite" from the [IPinfo Dashboard](https://ipinfo.io/account/data-downloads).
2. Extract the CSV to `./data/ipinfo_lite.csv` and place the official MMDB at `./data/ipinfo_lite.mmdb`.
3. Run `make generate` to build every artifact from those local sources without downloading them again.
4. The output files will be in `./data/`.

## Data Source

- [IPinfo Lite](https://ipinfo.io/lite) - official free country and ASN data for IPv4 and IPv6
- Official pipeline inputs: [`ipinfo_lite.csv.gz`](https://ipinfo.io/data/ipinfo_lite.csv.gz) and [`ipinfo_lite.mmdb`](https://ipinfo.io/data/ipinfo_lite.mmdb) (IPinfo token required)

IP address data in every distributed format is powered by
[IPinfo](https://ipinfo.io/lite).

## License

Code is licensed under [MIT](LICENSE).

Data is licensed under [CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/).
