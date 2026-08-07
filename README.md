# ipinfo-csv-lite

A lightweight IPinfo CSV database, updated daily at 12:00 UTC.

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

You can also download the latest release from [Releases](https://github.com/Alice39/ipinfo-csv-lite/releases/latest) page.

# Data Structure

## Python

```python
from typing import TypedDict, Optional

class IPInfo(TypedDict):
    cidr: str
    country_code: str
    continent_code: str
    as_number: int
    as_name: Optional[str]
```

## Go

```go
type IPInfo struct {
    CIDR          string  `json:"cidr"`
    CountryCode   string  `json:"country_code"`
    ContinentCode string  `json:"continent_code"`
    ASNumber      int     `json:"as_number"`
    ASName        *string `json:"as_name"`
}
```

## TypeScript

```typescript
interface IPInfo {
  cidr: string;
  country_code: string;
  continent_code: string;
  as_number: number;
  as_name: string | null;
}
```

## MMDB Record

The `.mmdb` file (MaxMind DB format, readable by any [maxminddb client](https://github.com/maxmind?utf8=%E2%9C%93&q=MaxMind-DB&type=all)) stores this structure per network:

```json
{
  "country_code": "US",
  "continent_code": "NA",
  "as_number": 15169,
  "as_name": "Google LLC"
}
```

Empty fields are omitted; `as_number` (uint32) is always present.

## XDB Region

The `.xdb` files follow the [ip2region xdb v3 format](https://github.com/lionsoul2014/ip2region), usable with any xdb searcher client. Each segment's region string is:

```
country_code|continent_code|as_number|as_name
```

## Requirements

- Go 1.24+ (only needed to build the pipeline; downloading the CSV requires nothing)

## Usage

### Automatic (requires an IPinfo token)

```bash
export IPINFO_TOKEN=<your-token>
make
```

This runs the full pipeline in sequence: `update` (download + extract) → `process` (reduce to the lite schema) → `release` (gzip/xz/zstd compression) → `mmdb` → `xdb` → `checksum`. Outputs land in `./data/`. Individual steps are available as `make update`, `make process`, etc.; checks via `make vet test`.

### Manual (no token)

1. Download the latest IPinfo CSV file from [IPinfo Dashboard](https://ipinfo.io/account/data-downloads) - "Free IP to Country + IP to ASN".
2. Move the CSV file to `./data/country_asn.csv`.
3. Run `make process release mmdb xdb checksum`.
4. The output files will be in `./data/`.

## Data Source

- [IPinfo](https://ipinfo.io/) - Free IP to Country + IP to ASN

## License

Code is licensed under [MIT](LICENSE).

Data is licensed under [CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/).
