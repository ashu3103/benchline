import logging
import atexit
import finder
import injector
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

def injectBenchmarkCodeWrapper(file: str) -> int:
    return injector.processFile(file)




