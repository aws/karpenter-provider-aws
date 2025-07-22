#!/usr/bin/env python3
"""
Parse _redirects file and convert to AWS Amplify JSON format
"""
import json
import sys
import os

def main():
    redirects_file = "website/static/_redirects"
    if not os.path.exists(redirects_file):
        print(f"Error: {redirects_file} not found", file=sys.stderr)
        sys.exit(1)
    rules = []
    try:
        with open(redirects_file, 'r') as f:
            for line_num, line in enumerate(f, 1):
                line = line.strip()     
                # Skip empty lines and comments
                if not line or line.startswith('#'):
                    continue
                parts = line.split()
                if len(parts) == 2:
                    rules.append({
                        "source": parts[0],
                        "target": parts[1],
                        "status": "301"
                    })
                else:
                    print(f"Error: Invalid redirect format on line {line_num}: {line}", file=sys.stderr)
                    sys.exit(1)
    except Exception as e:
        print(f"Error reading {redirects_file}: {e}", file=sys.stderr)
        sys.exit(1)
    # Output compact JSON to stdout
    print(json.dumps(rules))

if __name__ == "__main__":
    main()
