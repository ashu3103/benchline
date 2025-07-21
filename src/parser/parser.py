"""
    The **parser** package takes the log path (output of go test), parses it and extracts the 
    relevent metrics out of it
"""

import logging
import os
import re
from typing import List

logger = logging.getLogger('benchline')

_FAIL_KEYWORD = "FAIL"
_PASS_KEYWORD = "ok"

_METRIC_DATA = []

_TOTAL_PACKAGES = 0

#### Regular Expression ####

_FAIL_VERDICT_PATTERN = r"FAIL\s+([^\s]+)\s+([\d.]+)s"
_PASS_VERDICT_PATTERN = r"ok\s+([^\s]+)\s+([\d.]+)s"

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

def _getTestCaseName(name: str) -> str:
    l = name.split('-')
    l.pop()
    return "-".join(l)


def _parseTestcase(pkg: Package, lines: List[str], i: int) -> int:
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

def _parsePackage(lines: List[str], i: int) -> int:
    global _FAIL_VERDICT_PATTERN
    global _PASS_VERDICT_PATTERN

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
            return _parseTestcase(pkg, lines, i-1)
        else:
            return -1
    elif (curr_line.startswith(_PASS_KEYWORD)):
        match = re.match(_PASS_VERDICT_PATTERN, curr_line)
        if (match):
            pkg_name, total_time = match.groups()
            pkg = Package(pkg_name, total_time)
            return _parseTestcase(pkg, lines, i-1)
        else:
            return -1
    else:
        return -1

def parseBenchmarkLogs(log: str) -> int:
    global _FAIL_KEYWORD
    global _PASS_KEYWORD
    global _TOTAL_PACKAGES
    # global _METRIC_DATA

    with open(log, 'r') as f:
        lines = f.readlines()

    lines = [line.strip() for line in lines]

    total_lines = len(lines) 
    i = (total_lines - 1)

    while i>=0:
        line = lines[i]
        ## global failure
        if (i == (total_lines - 1) and line == _FAIL_KEYWORD):
            print('[Parser] overall the tests failed')
        else:
            ## parse till the package is over
            idx = _parsePackage(lines, i)
            if (idx == -1):
                return 1
            
            i = idx
            _TOTAL_PACKAGES = _TOTAL_PACKAGES + 1

        i = i - 1

    return 0

parseBenchmarkLogs("/home/ashu3103/Desktop/benchline/src/log.txt")