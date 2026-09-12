import unittest
from instance_types import select_types


class InstanceTypesTest(unittest.TestCase):
    def test_ignores_unrequested_types_from_zcompute(self):
        rows = [
            {"InstanceType": "other", "VCpuInfo": {"DefaultVCpus": 99}, "MemoryInfo": {"SizeInMiB": 99}},
            {"InstanceType": "z2.xlarge", "VCpuInfo": {"DefaultVCpus": 4}, "MemoryInfo": {"SizeInMiB": 8192}},
        ]
        self.assertEqual(select_types(rows, ["z2.xlarge"]), {"z2.xlarge": {"vcpus": 4, "memory_gb": 8}})

    def test_rejects_missing_type(self):
        with self.assertRaises(ValueError):
            select_types([], ["missing"])

    def test_rejects_ambiguous_type(self):
        row = {"InstanceType": "same", "VCpuInfo": {"DefaultVCpus": 4}, "MemoryInfo": {"SizeInMiB": 8192}}
        with self.assertRaises(ValueError):
            select_types([row, row], ["same"])


if __name__ == "__main__":
    unittest.main()
