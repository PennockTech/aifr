Read the required Go version from `go.mod` (the line starting `go `);
if it is not `go1.26.1` then replace `go1.26.1` in the below with the version from `go.mod`.

Run this:

```bash
typeset -p NO_PROXY ;
oIFS="$IFS"; IFS=','; read -a nop1 <<<"$NO_PROXY"; nop2=(); for e in "${nop1[@]}"; do case "$e" in (*.google*.com) : ;; (*) nop2+=("$e");; esac; done ;
NO_PROXY="${nop2[*]}"; IFS="$oIFS" ;
export NO_PROXY GOTOOLCHAIN=go1.26.1 ;
go version ;
typeset -p NO_PROXY
```

and then in future shell invocations, put `GOTOOLCHAIN=go1.26.1` into the environ at the start;
you do NOT need to modify NO_PROXY or IFS, that was a one-shot for the initial download.
The required Go version should now be installed.

Look for a `CLAUDE.md` or `AGENTS.md` file in this directory and read those if
they exist.
