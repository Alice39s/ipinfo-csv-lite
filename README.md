# ipinfo-csv-lite

A lightweight IPinfo CSV database, containing only "cidr, country_code, continent_code, as_number, as_name" fields.

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

## Requirements

- Python 3.10+

## Usage

1. Download the latest IPinfo CSV file from [IPinfo Dashboard](https://ipinfo.io/account/data-downloads) - "Free IP to Country + IP to ASN".
2. Move the CSV file to `./libs/country_asn.csv`.
3. Run the script.
4. The output file will be `./libs/ipinfo-lite.csv`.

## Data Source

- [IPinfo](https://ipinfo.io/)

## License

Code is licensed under [MIT](LICENSE).

Data is licensed under [CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/).