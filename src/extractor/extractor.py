"""
    The **scraper** package takes the log path (output of go test), parses it and extracts the 
    relevent metrics out of it
"""

import logging
import os
import re
import json
from typing import List

logger = logging.getLogger('benchline')

_FAIL_KEYWORD = "FAIL"
_PASS_KEYWORD = "ok"

_TOTAL_PACKAGES = 0

#### Regular Expression ####

_FAIL_VERDICT_PATTERN = r"FAIL\s+([^\s]+)\s+([\d.]+)s"
_PASS_VERDICT_PATTERN = r"ok\s+([^\s]+)\s+([\d.]+)s"
_MISSING_VERDICT_PATTERN = r"?\s+([^\s]+)\s+([\d.]+)s"

_TESTCASE_PATTERN = r'^(Benchmark\S+)\s+\d+\s+[\d.]+\s+ns/op\s+(\d+)\s+B/op\s+(\d+)\s+allocs/op'

class TestCase:
    name: str = None
    num_of_allocs: int   = 0   # allocs/op
    bytes_allocated: int = 0   # B/op

    def __init__(self, name: str, num_of_allocs: int, bytes_allocated: int):
        self.name = name
        self.num_of_allocs = num_of_allocs
        self.bytes_allocated = bytes_allocated

    def __str__(self):
        return f'({self.name}, {self.bytes_allocated}, {self.num_of_allocs})'

class Package:
    name: str = None
    testcases: List[TestCase] = []
    total_time: float = 0           # in seconds

    def __init__(self, name: str, total_time: float):
        self.name = name
        self.total_time = total_time
        self.testcases = []

    def __str__(self):
        out = f"({self.name}, {self.total_time}, ["
        for t in self.testcases:
            out = out + t.__str__() + ","

        out = out + "])"
        return out

    def addTestcase(self, testcase: TestCase):
        self.testcases.append(testcase)

_METRIC_DATA: List[Package] = []

def _getTestCaseName(name: str) -> str:
    l = name.split('-')
    l.pop()
    return "-".join(l)


def _extractTestcase(pkg: Package, lines: List[str], i: int) -> int:
    global _TESTCASE_PATTERN
    global _FAIL_VERDICT_PATTERN
    global _PASS_VERDICT_PATTERN

    while ((i>=0) and not (re.search(_FAIL_VERDICT_PATTERN, lines[i]) or re.search(_PASS_VERDICT_PATTERN, lines[i]))):
        match = re.match(_TESTCASE_PATTERN, lines[i])
        if (match):
            testname, bytes_per_alloc, num_of_allocs = match.groups()
            testcase = TestCase(_getTestCaseName(testname), num_of_allocs, bytes_per_alloc)
            pkg.addTestcase(testcase)
        i = i - 1

    return i+1

def _extractPackage(lines: List[str], i: int) -> int:
    global _FAIL_VERDICT_PATTERN
    global _PASS_VERDICT_PATTERN
    global _MISSING_VERDICT_PATTERN
    global _TOTAL_PACKAGES
    global _METRIC_DATA

    ## Base Case
    if i<0: return -1

    curr_line = lines[i]

    if (curr_line.startswith(_FAIL_KEYWORD)):
        ## can be static/runtime failure
        ## currently handle runtime
        match = re.match(_FAIL_VERDICT_PATTERN, curr_line)
        if (match):
            pkg_name, total_time = match.groups()
            pkg = Package(pkg_name, total_time)
            idx = _extractTestcase(pkg, lines, i-1)
            if (idx != -1):
                _METRIC_DATA.append(pkg)
            _TOTAL_PACKAGES = _TOTAL_PACKAGES + 1
            return idx
        else:
            return -1
    elif (curr_line.startswith(_PASS_KEYWORD)):
        match = re.match(_PASS_VERDICT_PATTERN, curr_line)
        if (match):
            pkg_name, total_time = match.groups()
            pkg = Package(pkg_name, total_time)
            idx = _extractTestcase(pkg, lines, i-1)
            if (idx != -1):
                _METRIC_DATA.append(pkg)
            _TOTAL_PACKAGES = _TOTAL_PACKAGES + 1
            return idx
        else:
            return -1
    elif (curr_line.startswith('?')):
        return i
    else:
        return -1

def extractBenchmarkLogs(log: str) -> int:
    global _FAIL_KEYWORD
    global _PASS_KEYWORD
    global _TOTAL_PACKAGES
    global _METRIC_DATA

    # Reset the global states
    _TOTAL_PACKAGES = 0
    _METRIC_DATA = []

    with open(log, 'r') as f:
        lines = f.readlines()

    lines = [line.strip() for line in lines]

    total_lines = len(lines) 
    i = (total_lines - 1)

    while i>=0:
        line = lines[i]
        ## global failure
        if (i == (total_lines - 1) and line == _FAIL_KEYWORD):
            logger.info('[Extractor] overall the tests failed')
        else:
            ## parse till the package is over
            idx = _extractPackage(lines, i)
            if (idx == -1):
                return 1
            
            i = idx

        i = i - 1

    logger.info(f'[Extractor] {_TOTAL_PACKAGES} packages found')
    return 0

def dumpMetrics(json_file_path: str):
    global _METRIC_DATA
    global _TOTAL_PACKAGES

    total_time: float = 0
    metrics = {}
    metrics['packages'] = []

    for met in _METRIC_DATA:
        pkgs = {}
        pkgs['package_name'] = met.name
        pkgs['pkg_total_time'] = f'{met.total_time}s'
        total_time = total_time + float(met.total_time)
        pkgs['testcases'] = []
        for t in met.testcases:
            tsts = {}
            tsts['function_name'] = t.name
            tsts['number_of_allocations'] = t.num_of_allocs
            tsts['bytes_allocated'] = f'{t.bytes_allocated}B'
            pkgs['testcases'].append(tsts)
        
        metrics['packages'].append(pkgs)

    metrics['total_packages'] = _TOTAL_PACKAGES
    metrics['total_time'] = f'{total_time}s'

    if json_file_path:
        with open(json_file_path, 'w') as json_file:
            json.dump(metrics, json_file, indent=4)