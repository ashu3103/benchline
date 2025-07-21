"""
    The **runner** package is pretty simple, it will just take the benchmark codes and execute
    the traditional `go test...` command over the project directory and output the result in a log
    file which will then be passed to analyser
"""

import os
import subprocess
import logging
from typing import Optional

logger = logging.getLogger('benchline')

def runBenchmarkTests(directory: str, output_log: Optional[str] = None) -> int:
    """
    Runs `go test -bench . -benchmem` inside the given directory and logs the results.

    :param directory: Path to the directory containing Go benchmark tests
    :param output_log: Optional path to save raw benchmark output
    """
    if not os.path.isdir(directory):
        logger.error(f"[Runner] directory '{directory}' does not exist.")
        return 1

    logger.info(f"[Runner] running Go benchmarks in: {directory}")

    cmd = ["go", "test", "-bench=.", "-benchmem"]
    try:
        result = subprocess.run(
            cmd,
            cwd=directory,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            check=False,
        )

        if output_log:
            with open(output_log, "w") as f:
                f.write(result.stdout)
            logger.info(f"[Runner] raw benchmark output saved to: {output_log}")

        return 0
    except FileNotFoundError:
        logger.error("[Runner] go toolchain not found. Make sure 'go' is installed and in PATH.")
        return 1
    except Exception as e:
        logger.exception(f"[Runner] benchmark execution failed: {e}")
        return 1

