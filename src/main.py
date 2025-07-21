import logging
import atexit
import finder
import injector
import runner
import parser
from typing import List

def usage():
    print('usage')
    pass

def version():
    print('version')
    pass

def initLogger():
    formatter = logging.Formatter(
        fmt='%(asctime)s %(levelname)s %(filename)s:%(lineno)d %(message)s',
        datefmt='%Y-%m-%d %H:%M:%S'
    )
    logger = logging.getLogger('benchline')
    logger.setLevel(logging.DEBUG)

    ## Get appropriate handler
    hndlr = logging.StreamHandler()
    hndlr.setFormatter(formatter)

    logger.addHandler(hndlr)
    return logger

logger = initLogger()
# Register a clean logging shutdown
atexit.register(logging.shutdown)

## Find the test files
def findTestFilesWrapper(root: str) -> List[str]:
    return finder.findTestFiles(root)

## Inject Benchmark Code
def injectBenchmarkCodeWrapper(file: str) -> int:
    return injector.processFile(file)

def runBenchmarkTestWrapper(project_directory: str, log_file: str) -> int:
    return runner.runBenchmarkTests(project_directory, log_file)

def parseBenchmarkLogs(log_file: str) -> str:
    return parser.parseBenchmarkLogs(log_file)


