package tftpserver

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/inngest/shoelaces/log"
	tftp "github.com/pin/tftp/v3"
)

type Server struct {
	core     *tftp.Server
	addr     string
	root     string
	readonly bool
	timeout  time.Duration
	logger   log.Logger
}

func New(addr, root string, readonly bool, timeout time.Duration) *Server {
	s := &Server{
		addr:     addr,
		root:     root,
		readonly: readonly,
		timeout:  timeout,
		logger:   log.MakeLogger(io.Discard).With("component", "tftp"),
	}

	read := func(filename string, rf io.ReaderFrom) error {
		path := s.safeJoin(filename)
		s.logger.Debug("TFTP read request", "filename", filename, "path", path)
		f, err := os.Open(path)
		if err != nil {
			s.logger.Error("Failed to open TFTP file for reading", "filename", filename, "path", path, "err", err)
			return err
		}
		defer func() { _ = f.Close() }()

		// Advertise transfer size if known (helps some PXE ROMs).
		if fi, err := f.Stat(); err == nil {
			s.logger.Debug("Advertised TFTP file size", "filename", filename, "path", path, "bytes", fi.Size())
			if ot, ok := rf.(tftp.OutgoingTransfer); ok {
				ot.SetSize(fi.Size())
			}
		}

		n, err := rf.ReadFrom(f)
		if err != nil {
			s.logger.Debug("TFTP read transfer returned error", "filename", filename, "path", path, "bytes", n, "err", err)
			return err
		}
		s.logger.Debug("TFTP read transfer completed", "filename", filename, "path", path, "bytes", n)
		return err
	}

	var write func(string, io.WriterTo) error
	if s.readonly {
		write = nil // disable uploads
	} else {
		write = func(filename string, wt io.WriterTo) error {
			path := s.safeJoin(filename)
			s.logger.Debug("TFTP write request", "filename", filename, "path", path)
			// O_EXCL prevents overwriting boot loaders accidentally.
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
			if err != nil {
				s.logger.Error("Failed to open TFTP file for writing", "filename", filename, "path", path, "err", err)
				return err
			}
			defer func() { _ = f.Close() }()
			n, err := wt.WriteTo(f)
			if err != nil {
				s.logger.Debug("TFTP write transfer returned error", "filename", filename, "path", path, "bytes", n, "err", err)
				return err
			}
			s.logger.Debug("TFTP write transfer completed", "filename", filename, "path", path, "bytes", n)
			return err
		}
	}

	core := tftp.NewServer(read, write)
	if s.timeout > 0 {
		core.SetTimeout(s.timeout)
	}
	core.SetHook(tftpHook{logger: s.logger})
	s.core = core
	return s
}

// WithLogger attaches a logger to server callbacks and transfer hooks.
func (s *Server) WithLogger(logger log.Logger) *Server {
	if logger == nil {
		logger = log.MakeLogger(io.Discard)
	}
	s.logger = logger.With("component", "tftp")
	if s.core != nil {
		s.core.SetHook(tftpHook{logger: s.logger})
	}
	return s
}

func (s *Server) ListenAndServe() error {
	if s.root == "" {
		return errors.New("tftp: root directory is empty")
	}
	s.logger.Info("Starting TFTP server", "addr", s.addr, "root", s.root, "readonly", s.readonly, "timeout", s.timeout)
	return s.core.ListenAndServe(s.addr) // blocks until Shutdown()
}

func (s *Server) Shutdown() { s.core.Shutdown() }

// safeJoin prevents directory traversal outside the TFTP root.
func (s *Server) safeJoin(name string) string {
	clean := filepath.Clean("/" + name)
	clean = strings.TrimPrefix(clean, "/")
	return filepath.Join(s.root, clean)
}

type tftpHook struct {
	logger log.Logger
}

func (h tftpHook) OnSuccess(stats tftp.TransferStats) {
	h.logger.Info("TFTP transfer succeeded", tftpTransferAttrs(stats)...)
}

func (h tftpHook) OnFailure(stats tftp.TransferStats, err error) {
	args := append(tftpTransferAttrs(stats), "err", err)
	if isTFTPOptionNegotiationAbort(err) {
		h.logger.Debug("TFTP transfer aborted by client during option negotiation", args...)
		return
	}
	h.logger.Warn("TFTP transfer failed", args...)
}

func tftpTransferAttrs(stats tftp.TransferStats) []any {
	args := []any{
		"remote_addr", stats.RemoteAddr.String(),
		"filename", stats.Filename,
		"mode", stats.Mode,
		"duration", stats.Duration,
		"datagrams_sent", stats.DatagramsSent,
		"datagrams_acked", stats.DatagramsAcked,
	}
	if len(stats.Opts) > 0 {
		args = append(args, "options", stats.Opts)
	}
	return args
}

func isTFTPOptionNegotiationAbort(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "sending block 0:") &&
		strings.Contains(msg, "code=8") &&
		strings.Contains(strings.ToLower(msg), "user aborted")
}
