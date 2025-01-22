import os
import gzip
import urllib.request
from datetime import date

def ensure_dir(dir_path):
    os.makedirs(dir_path, exist_ok=True)

def get_token():
    """Read token from environment variable"""
    token = os.getenv('IPINFO_TOKEN')
    if not token:
        print("Error: IPINFO_TOKEN environment variable is not set")
        return None
    return token

def check_version(libs_dir):
    """Check version file"""
    current_date = date.today().isoformat()
    version_file = os.path.join(libs_dir, 'ipinfo.version')
    
    if os.path.exists(version_file):
        with open(version_file) as f:
            stored_date = f.read().strip()
            if stored_date == current_date:
                print(f"Database is already up to date for {current_date}")
                return False
    return current_date

def download_database(token, libs_dir, current_date):
    """Download and extract database"""
    temp_file = os.path.join(libs_dir, 'country_asn.csv.gz')
    final_file = os.path.join(libs_dir, 'country_asn.csv')
    version_file = os.path.join(libs_dir, 'ipinfo.version')
    url = f"https://ipinfo.io/data/free/country_asn.csv.gz?token={token}"

    try:
        print("Downloading database...")
        urllib.request.urlretrieve(url, temp_file)

        # Extract file
        with gzip.open(temp_file, 'rb') as f_in:
            with open(final_file, 'wb') as f_out:
                f_out.write(f_in.read())
        
        # Write version file
        with open(version_file, 'w') as f:
            f.write(current_date)
        
        print("Database updated and extracted successfully")
        return True
        
    except Exception as e:
        print(f"Failed to process database: {e}")
        if os.path.exists(temp_file):
            os.remove(temp_file)
        return False
    finally:
        # Clean up temporary file
        if os.path.exists(temp_file):
            os.remove(temp_file)

def main():
    libs_dir = 'libs'
    ensure_dir(libs_dir)
    
    token = get_token()
    if not token:
        return 1
    
    current_date = check_version(libs_dir)
    if not current_date:
        return 0
        
    if download_database(token, libs_dir, current_date):
        return 0
    return 1

if __name__ == "__main__":
    exit(main())