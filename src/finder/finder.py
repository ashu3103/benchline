import os
from pathlib import Path
from typing import List

"""
    The finder package will take a go project directory, detect all the test files (unit + benchmark)
    and return a list of paths to the test path
        - Check the standard test subdirectories like - ('test/' or 'tests/')
        - If no such subdirectory is found, perform a DFS over the whole project directory
"""

_EXCLUDED_DIRS = {"vendor", "testdata", ".git", "third_party", "node_modules", "bin", "out"}
_STANDARD_TEST_DIRS = ['test', 'tests']

def _isDirectory(path: str) -> bool:
    return os.path.isdir(path)

def _isExcluded(dirname: str) -> bool:
    return dirname in _EXCLUDED_DIRS

def _doFindTestFiles(root: str) -> List[str]:
    test_files = []
    for dirpath, dirnames, filenames in os.walk(root):
        ## Skip excluded directories
        dirnames[:] = [d for d in dirnames if not _isExcluded(d)]

        for filename in filenames:
            ## Check if a Go test file
            if filename.endswith('_test.go'):
                full_path = os.path.abspath(os.path.join(dirpath, filename))
                test_files.append(full_path)

    return test_files


def findTestFiles(root: str) -> List[str]:
    if not _isDirectory(root):
        raise ValueError(f'{root} is not a valid directory')

    ## Check for standard test directories
    for test_dir in _STANDARD_TEST_DIRS:
        standard_test_dir = os.path.join(root, test_dir)
        if (_isDirectory(standard_test_dir)):
            return _doFindTestFiles(standard_test_dir)
    
    ## Fallback walk the whole project directory
    return _doFindTestFiles(root)
