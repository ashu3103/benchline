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

logger = logging.getLogger(__name__)

#### Constants Definitions ####

_UT_FUNCTION_SCOPE = 0
_UT_TEST_ARG_VARIABLE_NAME = None

## Tokens
CURLY_BRACE_OPEN  = '{'
CURLY_BRACE_CLOSE = '}'
PARENTHESIS_OPEN  = '('
PARENTHESIS_CLOSE = ')'
SPACE             = ' '
NEWLINE           = '\n'
TAB               = '\t'

## Keywords
FUNCTION_KEYWORD          = 'func'
UT_PARAMETER_TYPE_KEYWORD = '*testing.T'
BT_PARAMETER_TYPE_KEYWORD = '*testing.B'

#### Regex Definitions ####

FUNC_KEYWORD_DETECTION_PATTERN       = r'^func(?:\s+\S.*)?$'
FUNC_TEST_NAME_DETECTION_PATTERN     = r'(?:^|\S\s+)Test\S+\s*\(.*$'
FUNC_TEST_ARG_TYPE_DETECTION_PATTERN = r'\S+\s+\*testing\.T\s*[,)].*'
FUNC_BLOCK_START_DETECTION_PATTERN   = r'.*\)\s*\{\s*'

#### Test Cases ####
test_cases_func_detect = [
    "func",
    "func main()",
    "func   SomethingElse()",
    "func\tTabbedFunc()",
    "funcTest()",
    "x func y",
]

test_cases_func_name_detection = [
    "TestX(",
    "abc   TestX(",
    "func   TestSomething(",
    "func TestAbc   (",
    "func TestXyz\t (",
    "somePrefix TestDoSomething  (abc)",
    "Test (",
    "   TestX(",
    "func Test ("
]

test_cases_test_arg_detection = [
    "t *testing.T)",
    "myVar     *testing.T  )",
    "foo\t\t*testing.T, something else",
    "abc *testing.T  , trailing text",
    "*testing.T)",
    "   *testing.T)",
    "var*testing.T)",
]

test_cases_func_block_start = [
    ") }",
"func() )   {",
    "foo)     {",
    ")    {   ",
    "){",
    "){ ) }",
    ")   { // comment"
]

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

def _extract

def _isValidLine(line: str) -> bool:
    if (line == ''): return False
    return True

def _extractVariableName(line: str) -> str:
    l = line.strip()
    v = ""

    if line.count("("):
        v = l.split('(')[1]
    else:
        v = l
    return v.strip().split(',')[0].strip().split(' ')[0]

def _handleFunctionBlockStartDetection(line: str, reader: TextIO, writer: TextIO, output: str) -> bool:
    global _UT_FUNCTION_SCOPE
    global _UT_TEST_ARG_VARIABLE_NAME

    if (re.search(FUNC_BLOCK_START_DETECTION_PATTERN, line)):
        output = output + line + NEWLINE
        _UT_FUNCTION_SCOPE = _UT_FUNCTION_SCOPE + 1
        writer.write(output)
        return True
    else:
        next_line = next(reader)
        output = output + line + NEWLINE
        # writer.write(line + NEWLINE)

        processed_next_line = next_line[:].strip()
        ## find next valid line
        while(not _isValidLine(processed_next_line)):
            next_line = next(reader)
            processed_next_line = next_line[:].strip()
        
        if (re.search(FUNC_BLOCK_START_DETECTION_PATTERN, processed_next_line)):
            output = output + processed_next_line + NEWLINE
            # writer.write(processed_next_line)
            _UT_FUNCTION_SCOPE = _UT_FUNCTION_SCOPE + 1
            writer.write(output)
            return True
        
    _UT_TEST_ARG_VARIABLE_NAME = None
    return False

def _handleTestParameterDetection(line: str, reader: TextIO, writer: TextIO, output: str) -> bool:
    global _UT_TEST_ARG_VARIABLE_NAME
    ## Check if the current line has '*testing.T' if not get to next line
    if (re.search(FUNC_TEST_ARG_TYPE_DETECTION_PATTERN, line)):
        line = line.replace("*testing.T", "*testing.B", 1)
        _UT_TEST_ARG_VARIABLE_NAME = _extractVariableName(line)
        return _handleFunctionBlockStartDetection(line, reader, writer, output)
    else:
        next_line = next(reader)
        output = output + line + NEWLINE
        # writer.write(line + NEWLINE)

        processed_next_line = next_line[:].strip()
        ## find next valid line
        while(not _isValidLine(processed_next_line)):
            next_line = next(reader)
            processed_next_line = next_line[:].strip()
        
        if (re.search(FUNC_TEST_ARG_TYPE_DETECTION_PATTERN, processed_next_line)):
            processed_next_line = processed_next_line.replace("*testing.T", "*testing.B", 1)
            _UT_TEST_ARG_VARIABLE_NAME = _extractVariableName(processed_next_line)
            return _handleFunctionBlockStartDetection(processed_next_line, reader, writer, output)

    return False

def _handleTestFuncNameDetection(line: str, reader: TextIO, writer: TextIO, output: str) -> bool:
    ## Check if the current line has `TestXxx`` if not get the next valid line
    if (re.search(FUNC_TEST_NAME_DETECTION_PATTERN, line)):
        line = line.replace("Test", "Benchmark", 1)
        return _handleTestParameterDetection(line, reader, writer, output)
    else:
        next_line = next(reader)
        output = output + line + NEWLINE
        # writer.write(line + NEWLINE)

        processed_next_line = next_line[:].strip()
        ## find next valid line
        while(not _isValidLine(processed_next_line)):
            next_line = next(reader)
            processed_next_line = next_line[:].strip()
        
        if (re.search(FUNC_TEST_NAME_DETECTION_PATTERN, processed_next_line)):
            processed_next_line = processed_next_line.replace("Test", "Benchmark", 1)
            return _handleTestParameterDetection(processed_next_line, reader, writer, output)

    return False

def _detectUnitTestFunctionStatement(line: str, reader: TextIO, writer: TextIO) -> bool:
    processed_line = line[:].strip()
    output = ""

    if (re.search(FUNC_KEYWORD_DETECTION_PATTERN, processed_line)):
        return _handleTestFuncNameDetection(processed_line, reader, writer, output)
    return False

# TODO: If the function like `_detectUnitTestFunctionStatement` can read next line, the lineno may not be
# accuarte
def _injectBenchmarkCode(reader: TextIO, writer: TextIO) -> int:
    global _UT_TEST_ARG_VARIABLE_NAME
    global _UT_FUNCTION_SCOPE

    try:
        while True:
            line = next(reader)
            ## Scan till the unit test (TestXxx) function is encountered
            if (_detectUnitTestFunctionStatement(line, reader, writer)):
                logger.debug(f'unit test function detected')
                ## variable name must be present
                if not _UT_TEST_ARG_VARIABLE_NAME: 
                    return 1
                writer.write(f'\tfor {_UT_TEST_ARG_VARIABLE_NAME}.Loop() {CURLY_BRACE_OPEN}\n')
            else:
                if _UT_TEST_ARG_VARIABLE_NAME:
                    if (line.count(UT_PARAMETER_TYPE_KEYWORD)):
                        line = line.replace(UT_PARAMETER_TYPE_KEYWORD, BT_PARAMETER_TYPE_KEYWORD)
                    _UT_FUNCTION_SCOPE = _UT_FUNCTION_SCOPE + line.count(CURLY_BRACE_OPEN) - line.count(CURLY_BRACE_CLOSE)

                ## This marks the ending of a unit test function
                if (_UT_TEST_ARG_VARIABLE_NAME and _UT_FUNCTION_SCOPE == 0):
                    writer.write(f'\t{CURLY_BRACE_CLOSE}\n')
                    _UT_TEST_ARG_VARIABLE_NAME = None

                if (_UT_TEST_ARG_VARIABLE_NAME):
                    ## TODO: Skip some statements which are not relevnt in benchmark tests
                    writer.write(f'\t\t{line}')
                else:
                    writer.write(line)

    except StopIteration:
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
            return _injectBenchmarkCode(r, w)
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

if processFile("/home/ashu3103/Desktop/benchline/src/injector/tests/t1_test.go", ""):
    print("error")
