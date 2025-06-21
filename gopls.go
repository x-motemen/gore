package gore

import (
	"cmp"
	"context"
	"io"
	"os/exec"
	"path/filepath"
	"strings"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

type goplsCompleter struct {
	conn       jsonrpc2.Conn
	dir        string
	path       string
	source     string
	autoImport bool
	opened     []fileSource
}

type fileSource struct {
	path, source string
}

type rw struct {
	io.ReadCloser
	io.WriteCloser
}

func (rw rw) Close() error {
	return cmp.Or(rw.ReadCloser.Close(), rw.WriteCloser.Close())
}

func (c *goplsCompleter) init(dir, path, source string, autoImport bool) error {
	ctx := context.Background()

	cmd := exec.CommandContext(ctx, "gopls")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	c.conn = jsonrpc2.NewConn(jsonrpc2.NewStream(rw{stdout, stdin}))
	c.conn.Go(ctx, func(context.Context, jsonrpc2.Replier, jsonrpc2.Request) error {
		return nil
	})

	rootURI := protocol.DocumentURI("file://" + filepath.ToSlash(dir))
	initializeParams := protocol.InitializeParams{
		RootURI:               rootURI,
		Capabilities:          protocol.ClientCapabilities{},
		InitializationOptions: map[string]any{"completeUnimported": autoImport},
	}
	var initializeResponse protocol.InitializeResult
	if _, err := c.conn.Call(ctx, protocol.MethodInitialize,
		initializeParams, &initializeResponse); err != nil {
		return err
	}
	debugf("initializeResponse: %v", initializeResponse)

	if err := protocol.Call(ctx, c.conn, protocol.MethodInitialized, nil, nil); err != nil {
		return err
	}

	if err := c.open(path, source); err != nil {
		return err
	}

	c.dir, c.path, c.source, c.autoImport = dir, path, source, autoImport
	c.opened = nil // reset opened files (do not include the main file)
	return nil
}

func (c *goplsCompleter) open(path, source string) error {
	ctx := context.Background()

	debugf("open: %q: %q", path, source)
	fileURI := protocol.DocumentURI("file://" + filepath.ToSlash(path))
	didOpenTextDocumentParams := protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:  fileURI,
			Text: source,
		},
	}

	err := protocol.Call(ctx, c.conn, protocol.MethodTextDocumentDidOpen,
		didOpenTextDocumentParams, nil)
	if err != nil {
		return err
	}

	for i := range c.opened {
		if c.opened[i].path == path {
			c.opened[i].source = source
			return nil
		}
	}
	c.opened = append(c.opened, fileSource{path, source})
	return nil
}

func (c *goplsCompleter) reconnect() error {
	opened := c.opened
	if err := c.init(c.dir, c.path, c.source, c.autoImport); err != nil {
		return err
	}
	for _, f := range opened {
		if err := c.open(f.path, f.source); err != nil {
			return err
		}
	}
	return nil
}

func (c *goplsCompleter) update(source string) error {
	ctx := context.Background()

	select {
	case <-c.conn.Done():
		if err := c.reconnect(); err != nil {
			return err
		}
	default:
	}

	for c.source != source {
		i, j, k := diffString(c.source, source)
		debugf("update: %q", c.source[i:j])
		debugf("    --> %q", source[i:k])
		fileURI := protocol.DocumentURI("file://" + filepath.ToSlash(c.path))
		didChangeTextDocumentParams := protocol.DidChangeTextDocumentParams{
			TextDocument: protocol.VersionedTextDocumentIdentifier{
				TextDocumentIdentifier: protocol.TextDocumentIdentifier{
					URI: fileURI,
				},
			},
			ContentChanges: []protocol.TextDocumentContentChangeEvent{
				{
					Range: protocol.Range{
						Start: getPos(c.source, i),
						End:   getPos(c.source, j),
					},
					Text: source[i:k],
				},
			},
		}
		if err := protocol.Call(ctx, c.conn, protocol.MethodTextDocumentDidChange,
			didChangeTextDocumentParams, nil); err != nil {
			return err
		}
		c.source = source[:k] + c.source[j:]
	}

	return nil
}

func (c *goplsCompleter) complete(source string, pos int, exprMode bool) ([]string, int, error) {
	ctx := context.Background()

	if err := c.update(source); err != nil {
		return nil, 0, err
	}

	fileURI := protocol.DocumentURI("file://" + filepath.ToSlash(c.path))
	completionParams := protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: fileURI},
			Position:     getPos(source, pos),
		},
		Context: &protocol.CompletionContext{
			TriggerKind: protocol.CompletionTriggerKindInvoked,
		},
	}
	var completionList protocol.CompletionList
	if err := protocol.Call(ctx, c.conn, protocol.MethodTextDocumentCompletion,
		completionParams, &completionList); err != nil {
		return nil, 0, err
	}

	candidates := make([]string, 0, len(completionList.Items))
	for _, item := range completionList.Items {
		label := item.Label
		if item.Kind == protocol.CompletionItemKindKeyword ||
			item.Kind == protocol.CompletionItemKindFunction && label == printerName ||
			item.Kind == protocol.CompletionItemKindModule && label == "pp" ||
			strings.HasPrefix(label, "pp.") {
			continue
		}
		if exprMode &&
			(item.Kind == protocol.CompletionItemKindMethod ||
				item.Kind == protocol.CompletionItemKindFunction) {
			label += "("
		}
		candidates = append(candidates, label)
		pos = fromPos(source, item.TextEdit.Range.Start)
	}
	return candidates, pos, nil
}

func (c *goplsCompleter) close() error {
	if err := c.conn.Close(); err != nil {
		return err
	}
	<-c.conn.Done()
	return nil
}

func diffString(s, t string) (int, int, int) {
	var i, j int
	for s != "" {
		var u string
		if l := strings.IndexAny(s, "{ ;\n"); l >= 0 {
			u, s = s[:l+1], s[l+1:]
		} else {
			u, s = s, ""
		}
		if l := strings.Index(t, u); l > 0 && len(u) > 2 {
			return i, j, i + l
		} else if l == 0 {
			if i != j {
				return i, j, i
			}
			i += len(u)
			j = i
			t = t[len(u):]
		} else {
			j += len(u)
		}
	}
	return i, j, i + len(t)
}

func getPos(source string, pos int) protocol.Position {
	line := strings.Count(source[:pos], "\n")
	char := pos - strings.LastIndex(source[:pos], "\n") - 1
	return protocol.Position{Line: uint32(line), Character: uint32(char)}
}

func fromPos(source string, pos protocol.Position) int {
	lines := strings.SplitN(source, "\n", int(pos.Line)+1)
	return len(strings.Join(lines[:pos.Line], "")) +
		int(pos.Line) + int(pos.Character)
}
