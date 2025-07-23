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
import platform
import re
import subprocess
import logging
from typing import TextIO, Callable, List

logger = logging.getLogger('benchline')

#### Constants Definitions ####

_UT_FUNCTION_SCOPE = 0
_UT_TEST_ARG_VARIABLE_NAME = None

_IS_FILE_EXCLUDED = False

_CALLBACK_SCHEDULER: List[Callable[[str], None]] = []

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

IMPORT_KEYWORD_DETECTION_PATTERN     = r'^import\s+.*'
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

def _triggerTimeImport(file: str):
    with open(file, 'r') as f:
        lines = f.readlines()
    
    lines = [line.replace('import _ "time"', 'import "time"') for line in lines]

    with open(file, 'w') as f:
        f.writelines(lines)

def _handleDeadlineMethod(line: str) -> str:
    global _CALLBACK_SCHEDULER

    res = line.split('=')[0].split(':')[0]
    a1 = res.split(',')[0].strip()
    a2 = res.split(',')[1].strip()
    res = f"var {a1} time.Time" + NEWLINE
    if (a2 != '_'):
        res = res + f"{a2} := false" + NEWLINE

    _CALLBACK_SCHEDULER.append(_triggerTimeImport)
    return res

def _isDeadlineMethodPresent(line: str) -> bool:
    global _UT_TEST_ARG_VARIABLE_NAME
    return line.count(f"{_UT_TEST_ARG_VARIABLE_NAME}.Deadline()")

def _isParallelMethodPresent(line: str) -> bool:
    global _UT_TEST_ARG_VARIABLE_NAME
    return line.count(f"{_UT_TEST_ARG_VARIABLE_NAME}.Parallel()")

def _removeUnsupportedStatement(line: str):
    ## Simply skip the Parallel() method of t
    if (_isParallelMethodPresent(line)): 
        logger.debug("[Injector] 'Parallel()' method detected in unit tests")
        return "", True
    ## Handle Deadline() elegantly
    if _isDeadlineMethodPresent(line):
        logger.debug("[Injector] 'Deadline()' method detected in unit tests")
        return _handleDeadlineMethod(line), True
    
    return None, False

def _processLine(line: str) -> str:
    global _UT_FUNCTION_SCOPE
    global _UT_TEST_ARG_VARIABLE_NAME

    res = line[:]

    if _UT_TEST_ARG_VARIABLE_NAME:
        _UT_FUNCTION_SCOPE = _UT_FUNCTION_SCOPE + res.count(CURLY_BRACE_OPEN) - res.count(CURLY_BRACE_CLOSE)

    ## This marks the ending of a unit test function
    if (_UT_TEST_ARG_VARIABLE_NAME):
        if (_UT_FUNCTION_SCOPE == 0):
            res = f'\t{CURLY_BRACE_CLOSE}\n' + res
            _UT_TEST_ARG_VARIABLE_NAME = None
        else:
            tmp_res, present = _removeUnsupportedStatement(line)
            if (present and tmp_res != None):
                return tmp_res
            
            if (res.count(UT_PARAMETER_TYPE_KEYWORD)):
                res = res.replace(UT_PARAMETER_TYPE_KEYWORD, BT_PARAMETER_TYPE_KEYWORD)
            res = "\t\t" + res
    else:
        if (res.count(UT_PARAMETER_TYPE_KEYWORD)):
                res = res.replace(UT_PARAMETER_TYPE_KEYWORD, BT_PARAMETER_TYPE_KEYWORD)
        if (re.search(IMPORT_KEYWORD_DETECTION_PATTERN, res)):
            res = "import _ \"time\"" + NEWLINE + res

    return res

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
        # writer.write(output)
        return output, True
    else:
        next_line = next(reader)
        output = output + line + NEWLINE

        processed_next_line = next_line[:].strip()
        ## find next valid line
        while(not _isValidLine(processed_next_line)):
            next_line = next(reader)
            processed_next_line = next_line[:].strip()
        
        if (re.search(FUNC_BLOCK_START_DETECTION_PATTERN, processed_next_line)):
            output = output + processed_next_line + NEWLINE
            # writer.write(processed_next_line)
            _UT_FUNCTION_SCOPE = _UT_FUNCTION_SCOPE + 1
            # writer.write(output)
            return output, True
        
    _UT_TEST_ARG_VARIABLE_NAME = None
    output = output + next_line + NEWLINE
    return output, False

def _handleTestParameterDetection(line: str, reader: TextIO, writer: TextIO, output: str) -> bool:
    global _UT_TEST_ARG_VARIABLE_NAME
    ## Check if the current line has '*testing.T' if not get to next line
    if (re.search(FUNC_TEST_ARG_TYPE_DETECTION_PATTERN, line)):
        line = line.replace("*testing.T", "*testing.B", 1)
        _UT_TEST_ARG_VARIABLE_NAME = _extractVariableName(line)
        logger.debug(f"[Injector] testing argument name: {_UT_TEST_ARG_VARIABLE_NAME}")
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
            logger.debug(f"[Injector] testing argument name: {_UT_TEST_ARG_VARIABLE_NAME}")
            return _handleFunctionBlockStartDetection(processed_next_line, reader, writer, output)

    output = output + next_line + NEWLINE
    return output, False

def _handleTestFuncNameDetection(line: str, reader: TextIO, writer: TextIO, output: str) -> bool:
    global _IS_FILE_EXCLUDED
    ## Check if the current line has `TestXxx`` if not get the next valid line
    if (re.search(FUNC_TEST_NAME_DETECTION_PATTERN, line)):
        if _IS_FILE_EXCLUDED:
            line = line.replace("Test", "ExcludedTest", 1)
        else:
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

    output = output + next_line + NEWLINE
    return output, False

def _detectUnitTestFunctionStatement(line: str, reader: TextIO, writer: TextIO) -> bool:
    processed_line = line[:].strip()
    output = ""

    if (re.search(FUNC_KEYWORD_DETECTION_PATTERN, processed_line)):
        return _handleTestFuncNameDetection(processed_line, reader, writer, output)
    
    output = output + line
    return output, False

def _addExclusionBuildTags(reader: TextIO, writer: TextIO) -> int:
    writer.write("//go:build benchline_exclude\n")
    writer.write("// +build benchline_exclude\n")
    writer.write("\n")

    try:
        while True:
            line = next(reader)
            writer.write(line)

    except StopIteration:
        return 0

# TODO: If the function like `_detectUnitTestFunctionStatement` can read next line, the lineno may not be
# accuarte
def _injectBenchmarkCode(reader: TextIO, writer: TextIO) -> int:
    global _UT_TEST_ARG_VARIABLE_NAME
    global _UT_FUNCTION_SCOPE

    try:
        while True:
            line = next(reader)

            output, present = _detectUnitTestFunctionStatement(line, reader, writer)
            ## Scan till the unit test (TestXxx) function is encountered
            if (present):
                writer.write(output)
                logger.debug(f'[Injector] unit test function detected')
                ## variable name must be present
                if not _UT_TEST_ARG_VARIABLE_NAME: 
                    return 1
                writer.write(f'\tfor {_UT_TEST_ARG_VARIABLE_NAME}.Loop() {CURLY_BRACE_OPEN}\n')
            else:
                result = _processLine(output)
                writer.write(result)

    except StopIteration:
        return 0

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
def processFile(file_path: str, is_excluded: bool) -> int:
    global _CALLBACK_SCHEDULER
    global _UT_FUNCTION_SCOPE
    global _UT_TEST_ARG_VARIABLE_NAME
    global _IS_FILE_EXCLUDED

    ## Reset the global states
    _CALLBACK_SCHEDULER = []
    _IS_FILE_EXCLUDED = is_excluded
    _UT_FUNCTION_SCOPE = 0
    _UT_TEST_ARG_VARIABLE_NAME = None

    dev_arch = platform.machine()
    dev_os = platform.system()

    logger.debug(f"[Injector] system architecture: {dev_arch}")
    logger.debug(f"[Injector] system os: {dev_os}")

    if dev_arch.lower() == "amd64" or dev_arch.lower() == "x86_64":
        dev_arch = "amd64"
    elif dev_arch.lower() == "arm64" or dev_arch.lower() == "aarch64":
        dev_arch = "arm64"
    else:
        logger.error(f"[Injector] system architecture {dev_arch} not compatible")
        return 1
    
    if dev_os.lower() == "linux":
        dev_os = "linux"
    elif dev_os.lower() == "darwin":
        dev_os = "mac"
    else:
        logger.error(f"[Injector] system os {dev_os} not compatible")
        return 1


    gosyntax_bin = os.path.join(os.path.dirname(os.path.abspath(__file__)), "bin")
    gosyntax_bin = os.path.join(gosyntax_bin, f"syntaxchecker-{dev_os}-{dev_arch}")

    temp_path = file_path + '.tmp'
    """ 
        Read the actual file and write the derived results in a temporary file
    """
    try:
        with open(file_path, 'r', encoding="utf-8") as r, open(temp_path, 'w', encoding="utf-8") as w:
            if (_injectBenchmarkCode(r, w)): return 1
    except FileNotFoundError:
        logger.error(f'[Injector] File {file_path} not found')
        return 1
    except PermissionError:
        logger.error(f'[Injector] Permissions denied to read {file_path}')
        return 1
    except IOError as e:
        logger.error(f'[Injector] An I/O exception has occurred')
        return 1
    except Exception as e:
        logger.error(f'[Injector] An unexpected error occurred: {e}')
        return 1     

    """
        Schedule the deferred callbacks
    """
    for callbacks in _CALLBACK_SCHEDULER:
        callbacks(temp_path)

    """ 
        Check the syntax of the injected go test file
    """
    if _checkSyntax(temp_path, gosyntax_bin):
        logger.info(f'[Injector] benchmark test injection in {file_path} success')
        try:
            os.replace(temp_path, file_path)  # overwrite original file
        except OSError as e:
            logger.error(f'[Injector] An OS error occurred: {e}')
            return 1
    else:
        logger.warning(f'[Injector] benchmark test injection in {file_path} failed')
        try:
            os.remove(temp_path)
        except OSError as e:
            logger.error(f'[Injector] An OS error occurred: {e}')
            return 1

    return 0
