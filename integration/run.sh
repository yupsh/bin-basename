#!/bin/sh
# Integration checks for yup-basename, run inside a Debian (GNU coreutils)
# container.
#
# parity ARGS...  — yup-basename must produce byte-identical output to GNU
#                   `basename` for the same operands and flags.
# assert WANT ARGS...  — yup-basename must produce WANT exactly (used where no
#                   reference comparison is meaningful).
set -eu

fails=0

parity() {
	ours=$(yup-basename "$@" 2>/dev/null || true)
	gnu=$(basename "$@" 2>/dev/null || true)
	if [ "$ours" = "$gnu" ]; then
		printf 'ok    parity  basename %s\n' "$*"
	else
		printf 'FAIL  parity  basename %s\n        gnu:  %s\n        ours: %s\n' "$*" "$gnu" "$ours"
		fails=$((fails + 1))
	fi
}

assert() {
	want=$1
	shift
	got=$(yup-basename "$@" 2>/dev/null || true)
	if [ "$got" = "$want" ]; then
		printf 'ok    assert  basename %s\n' "$*"
	else
		printf 'FAIL  assert  basename %s\n        want: %s\n        got:  %s\n' "$*" "$want" "$got"
		fails=$((fails + 1))
	fi
}

# Plain path: strip leading directory components.
parity /usr/local/bin/script.sh
parity relative/path/file.txt
parity script.sh
parity /home/user/.bashrc

# NAME SUFFIX form: strip a trailing suffix operand.
parity /usr/local/bin/script.sh .sh
parity archive.tar.gz .tar.gz
# GNU never empties the name: a suffix equal to the whole name is kept.
parity .txt .txt

# Trailing slashes and degenerate paths.
parity /path/to/dir/
parity /
parity .
parity ..

# -a/--multiple: every operand is a NAME.
parity -a /x/y /p/q.txt
parity --multiple /a/b /c/d /e/f

# -s/--suffix: implies -a, strips the suffix from each NAME.
parity -s .txt a.txt /b/c.txt
parity --suffix .log /var/log/app.log /tmp/run.log

# -z/--zero: NUL-terminated records (compared verbatim, no shell trimming).
zours=$(yup-basename -az /x/y /p/q | od -An -c)
zgnu=$(basename -az /x/y /p/q | od -An -c)
if [ "$zours" = "$zgnu" ]; then
	printf 'ok    parity  basename -az /x/y /p/q (NUL)\n'
else
	printf 'FAIL  parity  basename -az /x/y /p/q (NUL)\n        gnu:  %s\n        ours: %s\n' "$zgnu" "$zours"
	fails=$((fails + 1))
fi

if [ "$fails" -ne 0 ]; then
	printf '\n%s check(s) failed\n' "$fails"
	exit 1
fi
printf '\nall checks passed\n'
