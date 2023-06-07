#!/bin/bash

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

cd $SCRIPT_DIR

./main -yourMySQLdatabasepassword "Usm@1?/#Qv^avF" >> main.log 2>&1 &
