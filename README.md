# gore [![Travis Build Status](https://travis-ci.org/motemen/gore.svg?branch=master)](https://travis-ci.org/motemen/gore)
### Yet another Go REPL that works nicely. Featured with line editing, code completion, and more.

![Screencast](doc/screencast.gif)

(Screencast taken with [cho45/KeyCast](https://github.com/cho45/KeyCast))

## Usage

```sh
gore
```
After a prompt is shown, enter any Go expressions/statements/functions or commands described below.

To quit the session, type `Ctrl-D` or use `:q` command.

## Features

- Line editing with history
- Multi-line input
- Package importing with completion
- Evaluates any expressions, statements and function declarations
- No "evaluated but not used" errors
- Code completion (requires [gocode](https://github.com/mdempsky/gocode))
- Showing documents
- Auto-importing (`gore -autoimport`)

## REPL Commands

Some functionalities are provided as commands in the REPL:

```
:import <package path>  Import package
:type <expr>            Print the type of expression
:print                  Show current source
:write [<filename>]     Write out current source to file
:clear                  Clear the codes
:doc <expr or pkg>      Show document
:help                   List commands
:quit                   Quit the session
```

## Installation

The gore command requires Go tool-chains on runtime, so standalone binary is not distributed.

```sh
go get -u github.com/motemen/gore/cmd/gore
```

Make sure `$GOPATH/bin` is in your `$PATH`.

Also recommended:

```sh
go get -u github.com/mdempsky/gocode   # for code completion
```

## FAQ/Caveats

- If you see `too many arguments in call to mainScope.LookupParent`
  while installing gore, run `go get -u golang.org/x/tools/go/types`.
- gore runs code using `go run` for each input. If you have entered
  time-consuming code, gore will run it for each input and take some time.

## License

[The MIT License](./LICENSE).

## Author

motemen &lt;<motemen@gmail.com>&gt;
