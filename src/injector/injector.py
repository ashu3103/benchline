"""
    The **injector** package will scan the whole Go test file and try to inject benchmark test function 
    for every unit test function found.

    After injecting the code, it will check if the modified file is syntactically correct using a 
    Go syntax checker binary (native), if syntax not correct, skip the file and no benchmark tests
    skip the file
    
    This will help to get benchmark metrics from the test functions.
"""
from enum import Enum
import os
import re
import subprocess
import logging
from typing import TextIO

#### Constants Definitions ####

## Tokens
CURLY_BRACE_OPEN  = '{'
CURLY_BRACE_CLOSE = '}'
SPACE             = ' '
NEWLINE           = '\n'

## Keywords
FUNCTION_KEYWORD  = 'func'

#### Regex Definitions ####

UT_FUNC_PATTERN      = r'func\s+Test\w*'
UT_FUNC_NAME_PATTERN = r'Test\w*'

#### Class Definitions ####

class Peekable:
    def __init__(self, iterable):
        self._iterator = iter(iterable)
        self._peeked = None
        self._has_peeked = False

    def peek(self) -> str:
        if not self._has_peeked:
            try:
                self._peeked = next(self._iterator)
                self._has_peeked = True
            except StopIteration:
                self._peeked = None
        return self._peeked

    def __iter__(self):
        return self

    def __next__(self):
        if self._has_peeked:
            self._has_peeked = False
            return self._peeked
        return next(self._iterator)

#### Function Definitions ####    

logger = logging.getLogger(__name__)

def _detectUnitTestFunctionStatement(line: str, reader: TextIO, writer: TextIO) -> bool:
    processed_line = line[:].strip()

    if (re.search(UT_FUNC_PATTERN, processed_line) and processed_line.endswith(CURLY_BRACE_OPEN)): # Handle: `func TestXxx(){`
        processed_line = processed_line.replace("Test", "Benchmark", 1)
        processed_line = processed_line.replace("testing.T", "testing.B")
        writer.write(processed_line + NEWLINE)
        return True
    elif (processed_line.startswith(FUNCTION_KEYWORD) and processed_line.endswith(FUNCTION_KEYWORD)): # Handle: `func\nTestXxx(){`
        next_line = next(reader)
        writer.write(processed_line + NEWLINE)

        processed_next_line = next_line[:].strip()

        if (re.match(UT_FUNC_NAME_PATTERN, processed_next_line)):
            processed_next_line = processed_next_line.replace("Test", "Benchmark", 1)
            processed_next_line = processed_next_line.replace("testing.T", "testing.B")
            writer.write(processed_next_line + NEWLINE)
            return True
        else:
            return False
        
    return False

def _doProcessFile(reader: TextIO, writer: TextIO) -> int:
    offset = 0
    lineno = 1

    try:
        while True:
            line = next(reader)
            ## Scan till the unit test (TestXxx) function is encountered
            if (_detectUnitTestFunctionStatement(line, reader, writer)):
                logger.debug(f'unit test function detected at line: {lineno}')
            else:
                writer.write(line)
            
            lineno = lineno + 1
            offset = offset + len(line)
    except StopIteration:
        pass


def _injectBenchmarkCode():
    pass

## Check the syntax of the go file using syntax checker binary
def _checkSyntax(path: str, gosyntax_bin: str) -> bool:
    with open(path, "rb") as f:
        proc = subprocess.run(
            [gosyntax_bin],
            input=f.read(),
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE
        )
    return proc.returncode == 0

## Process Go Test files
def processFile(file_path: str, gosyntax_bin: str) -> int:

    temp_path = file_path + '.tmp'
    """ 
        Read the actual file and write the derived results in a temporary file
    """
    try:
        with open(file_path, 'r', encoding="utf-8") as r, open(temp_path, 'w', encoding="utf-8") as w:
            return _doProcessFile(r, w)
    except FileNotFoundError:
        print(f'File {file_path} not found')
        return 1
    except PermissionError:
        print(f'Permissions denied to read {file_path}')
        return 1
    except IOError as e:
        print(f'An I/O exception has occurred')
        return 1
    except Exception as e:
        print(f'An unexpected error occurred: {e}')
        return 1     

    """ 
        Check the syntax of the injected go test file
    """
    # if _checkSyntax(temp_path, gosyntax_bin):
    #     logger.info(f'benchmark test injection in {file_path} success')
    #     try:
    #         os.replace(temp_path, file_path)  # overwrite original file
    #     except OSError as e:
    #         logger.error(f'An OS error occurred: {e}')
    #         return 1
    # else:
    #     logger.warning(f'benchmark test injection in {file_path} failed')
    #     try:
    #         os.remove(temp_path)
    #     except OSError as e:
    #         logger.error(f'An OS error occurred: {e}')
    #         return 1

    return 0


if processFile("/home/ashu3103/Desktop/benchline/src/injector/tests/t0_test.go", ""):
    print("error")