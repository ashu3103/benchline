"""
    The **finder** package will take a go project directory, detect all the test files and return 
    a list of paths to the test path

    This will first check the standard test subdirectories like - ('test/' or 'tests/') and if no
    such subdirectory is found, perform a DFS over the whole project directory to find test files
    (typically the files ending with '_test.go')

    Additionally we can exclude the directories which can contain static (large) data (think about 
    media files)
"""
import os
import logging
from typing import List

logger = logging.getLogger('benchline')

_EXCLUDED_DIRS = {"vendor", "testdata", ".git", "third_party", "node_modules", "bin", "out"}
_STANDARD_TEST_DIRS = ['test', 'tests']

def _isDirectory(path: str) -> bool:
    return os.path.isdir(path)

def _isExcluded(dirname: str) -> bool:
    return dirname in _EXCLUDED_DIRS

def _isGoTestFile(filename: str) -> bool:
    # TODO: Check if the file is valid (optional)
    return filename.endswith('_test.go')

def _doFindTestFiles(root: str) -> List[str]:
    test_files = []
    for dirpath, dirnames, filenames in os.walk(root):
        ## Skip excluded directories
        dirnames[:] = [d for d in dirnames if not _isExcluded(d)]

        for filename in filenames:
            ## Check if a Go test file
            if _isGoTestFile(filename):
                logger.debug(f'[Finder] go test file detected: {filename}')
                full_path = os.path.abspath(os.path.join(dirpath, filename))
                test_files.append(full_path)

    return test_files


def findTestFiles(root: str) -> List[str]:
    if not _isDirectory(root):
        logger.error(f'[Finder] {root} is not a valid directory')
        return []

    ## Check for standard test directories
    for test_dir in _STANDARD_TEST_DIRS:
        standard_test_dir = os.path.join(root, test_dir)
        if (_isDirectory(standard_test_dir)):
            return _doFindTestFiles(standard_test_dir)
    
    ## Fallback walk the whole project directory
    return _doFindTestFiles(root)
