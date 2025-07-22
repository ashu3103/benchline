import logging
import atexit
import finder
import injector
import runner
import sys
import os
import extractor
from pathlib import Path
from typing import List

repo_path: str = None
repo_log_path: str = None
repo_json_path: str = None

project_src_directory = os.path.abspath(os.getcwd())

def usage():
    print('benchline </path/to/project/pool>')

def _initLogger():
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

def _getProjectName(path: str) -> str:
    l = path.split('/')
    if (path[len(path) - 1] == '/'):
        return l[len(l) - 2]
    return l[len(l) - 1]

## Find the test files
def _findTestFilesWrapper(root: str) -> List[str]:
    return finder.findTestFiles(root)

## Inject Benchmark Code
def _injectBenchmarkCodeWrapper(files: List[str]) -> int:
    for file in files:
        res = injector.processFile(file)
        if res: 
            logger.error(f"[Benchline] error injecting code in {file}")
            return 1
    return 0

def _runBenchmarkTestWrapper(project_directory: str, log_file: str) -> int:
    return runner.runBenchmarkTests(project_directory, log_file)

def _extractBenchmarkLogsWrapper(log_file: str) -> int:
    return extractor.extractBenchmarkLogs(log_file)

def _dumpMetricsWrapper(json_file: str) -> None:
    return extractor.dumpMetrics(json_file)

def _runPipeline(go_dir: str) -> int:
    project_name = _getProjectName(go_dir)
    logger.debug(f"[Benchline] project name: {project_name}")

    test_files: List[str] = _findTestFilesWrapper(go_dir)
    if (len(test_files) == 0):
        logger.info(f"[Benchline] no test file found in {go_dir}")
        return 0
    
    if (_injectBenchmarkCodeWrapper(test_files)):
        logger.error(f"[Benchline] error injecting benchmark code in {go_dir}")

    if not repo_log_path:
        logger.error("[Benchline] repo log path not set")
        return 1
    log_file = os.path.join(repo_log_path, project_name)

    if (_runBenchmarkTestWrapper(go_dir, log_file)):
        logger.error(f"[Benchline] error running benchmark test in {go_dir}")

    if (_extractBenchmarkLogsWrapper(log_file)):
        logger.error(f"[Benchline] error extracting metric for {go_dir}")
    
    if not repo_json_path:
        logger.error("[Benchline] repo json path not set")
        return 1
    json_file = os.path.join(repo_json_path, project_name)

    _dumpMetricsWrapper(json_file)

logger = _initLogger()
# Register a clean logging shutdown
atexit.register(logging.shutdown)

if __name__ == "__main__":
    if (len(sys.argv) != 2):
        usage()
        sys.exit(1)

    dir_pool = sys.argv[1]
    if (not os.path.isdir(dir_pool)):
        logger.error(f"[Benchline] {dir_pool} is not a directory...Exiting")
        sys.exit(1)

    ## Create a repo subdirectory
    r = os.path.join(project_src_directory, 'repo')
    try:
        os.mkdir(r)
        repo_path = r
        print(f"[Benchline] directory '{r}' created successfully.")
    except FileExistsError:
        repo_path = r
        print(f"[Benchline] directory '{r}' already exists.")
    except Exception as e:
        print(f"[Benchline] an error occurred: {e}")

    rl = os.path.join(r, 'log')
    try:
        os.mkdir(rl)
        repo_log_path = rl
        print(f"[Benchline] directory '{rl}' created successfully.")
    except FileExistsError:
        repo_log_path = rl
        print(f"[Benchline] directory '{rl}' already exists.")
    except Exception as e:
        print(f"[Benchline] an error occurred: {e}")
    
    rj = os.path.join(r, 'json')
    try:
        os.mkdir(rj)
        repo_json_path = rj
        print(f"[Benchline] directory '{rj}' created successfully.")
    except FileExistsError:
        repo_json_path = rj
        print(f"[Benchline] directory '{rj}' already exists.")
    except Exception as e:
        print(f"[Benchline] an error occurred: {e}")
    
    ## Scan the directory pool
    dir_pool = Path(dir_pool)
    for go_project in dir_pool.iterdir():
        if go_project.is_dir():
            go_mod_file = go_project / "go.mod"
            if go_mod_file.exists():
                logger.info(f"[Benchline] {go_project} is a go project")
                ## Push into pipeline
                go_poject_path = str(go_project)
                if (_runPipeline(go_poject_path)):
                    logger.error(f"[Benchline] pipeline fail for {go_project}")
            else:
                logger.warning(f"[Benchline] {go_project} is not a go project")
                continue
