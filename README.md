# go-eigentrust

Go implementation of
the [EigenTrust](https://nlp.stanford.edu/pubs/eigentrust.pdf) algorithm.
Comes with both the server and the client implementation.

## Installation

Make sure Go 1.20+ is installed,
e.g. using [gimme](https://github.com/travis-ci/gimme):

```shell
eval $(gimme 1.20)
```

Then install the `eigentrust` binary into `$GOBIN` (typically `$HOME/go/bin`):

```shell
go install k3l.io/go-eigentrust/cmd/eigentrust@latest
```

Or from a local clone of this repository:

```shell
go install ./cmd/eigentrust
```

Try running `eigentrust` with no arguments:

```shell
eigentrust
```

If you see the help message, your `$PATH` is already set up correctly.
If you get a command-not-found error, you need to add `$GOBIN` to `$PATH` (see
*Appendix > Updating `$PATH`* at the end of this README).

## Running Server

```shell
eigentrust serve
```

## Using Compute Client CLI

This requires a running server.
See the *Running Server* section above to run one locally.

### Input

You will need *local trust* as well as *pre-trust.*  Both can be specified
using CSV.

Sample local trust (`lt.csv`):

```csv
from,to,value
ek,sd,100
vm,sd,100
ek,vm,75
```

Here we have 3 peers: EK, VM, and SD.
Both EK and VM trust SD by 100.
EK also trusts VM, by 3/4 of how much he trusts SD.

Sample pre-trust (`pt.csv`):

```csv
peer_id,value
ek,50
vm,100
```

Here, both EK and VM are pre-trusted by the network (*a priori* trust).
VM is trusted twice as much as EK.

### Running CLI

To run EigenTrust using the above input:

```shell
eigentrust basic compute -L -l lt.csv -p pt.csv
```

Outputs:
```csv
ek,0.21705427907166472
sd,0.3023255429103218
vm,0.4806201780180134
```

Here, the EigenTrust algorithm distributed the network's trust onto the 3 peers:

* EK gets 21.7%
* SD gets 30.2%
* VM gets 48.1%

## Appendix

### Tweaking Alpha

The pre-trust input defines the *relative* ratio
by which the network distributes its *a priori* trust onto trustworthy peers,
in this case EK and VM.

You can also tweak the overall *absolute* strength of the pre-trust.
This parameter, named *alpha*,
represents the portion of the EigenTrust output taken from the pre-trust.
For example, with alpha of 0.2, the EigenTrust output is a blend of 20%
pre-trust and 80% peer-to-peer trust.

The CLI default for alpha is 0.5 (50%).  If you re-run EigenTrust using a lower
alpha of only 0.01 (1%):

```shell
eigentrust basic compute -L -l lt.csv -p pt.csv -a 0.01
```

We get a different result:

```csv
ek,0.16536739107782936
sd,0.4401132971693594
vm,0.39451931175281096
```

EK and VM's trust shares got lower (EK 21.7% ⇒ 16.5%, VM 48.1% ⇒ 39.5%),
whereas SD's trust share soared (30.2% ⇒ 44%) despite not being pre-trusted.
This is because, with only 1% pre-trust level,
the peer-to-peer trust opinions (where SD is trusted by both EK and VM)
make up for a much larger portion of trust.

### Updating `$PATH`

For Bourne shell compatibles (sh/bash/zsh/…), add this to `~/.profile`,
and restart the shell you are using for this tutorial
(or run the same command in the shell directly).

```sh
PATH="${PATH+"${PATH}:"}${GOBIN:-"${GOPATH:-"${HOME}/go"}/bin"}"
```

For C shell and compatibles (csh/tcsh/…), add this to `~/.login`,
and restart the shell you are using for this tutorial
(or run the same commands in the shell directly).

```csh
if (! $?GOPATH) setenv GOPATH ~/go
if (! $?GOBIN) setenv GOBIN "${GOPATH}/bin"
set path = ($path $GOBIN)
```

## Acknowledgments

* This project is tested with BrowserStack - https://www.browserstack.com/
