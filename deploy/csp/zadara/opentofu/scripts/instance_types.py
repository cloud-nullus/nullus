#!/usr/bin/env python3
"""Read zCompute's catalog without relying on its ignored instance-type filter."""
import json
import subprocess
import sys


def select_types(rows, requested):
    selected = {}
    for name in requested:
        matches = [row for row in rows if row.get("InstanceType") == name]
        if len(matches) != 1:
            raise ValueError(f"Expected exactly one catalog entry for {name}; got {len(matches)}")
        row = matches[0]
        selected[name] = {
            "vcpus": row["VCpuInfo"]["DefaultVCpus"],
            "memory_gb": row["MemoryInfo"]["SizeInMiB"] / 1024,
        }
    return selected


def main():
    query = json.load(sys.stdin)
    # D6: zCompute ignores the instance-type filter used by AWS provider 3.33.
    # Cost: one full catalog read per plan. Remove when the provider/API is fixed.
    result = subprocess.run(
        ["aws", "--profile", query["profile"], "--region", query["region"],
         "--endpoint-url", query["endpoint"], "--no-cli-pager", "ec2",
         "describe-instance-types", "--output", "json"],
        capture_output=True, text=True, timeout=60, check=False,
    )
    if result.returncode:
        raise ValueError(f"AWS CLI catalog read failed (exit {result.returncode}); check profile/endpoint access")
    catalog = select_types(json.loads(result.stdout)["InstanceTypes"], json.loads(query["types"]))
    print(json.dumps({"catalog_json": json.dumps(catalog)}))


if __name__ == "__main__":
    try:
        main()
    except (ValueError, KeyError, OSError, subprocess.TimeoutExpired) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
