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
	}

	read := func(filename string, rf io.ReaderFrom) error {
		path := s.safeJoin(filename)
		s.debug("TFTP read request", "filename", filename, "path", path)
		f, err := os.Open(path)
		if err != nil {
			s.error("Failed to open TFTP file for reading", "filename", filename, "path", path, "err", err)
			return err
		}
		defer func() { _ = f.Close() }()

		// Advertise transfer size if known (helps some PXE ROMs).
		if fi, err := f.Stat(); err == nil {
			s.debug("Advertised TFTP file size", "filename", filename, "path", path, "bytes", fi.Size())
			if ot, ok := rf.(tftp.OutgoingTransfer); ok {
				ot.SetSize(fi.Size())
			}
		}

		n, err := rf.ReadFrom(f)
		if err != nil {
			s.error("TFTP read transfer failed", "filename", filename, "path", path, "bytes", n, "err", err)
			return err
		}
		s.debug("TFTP read transfer completed", "filename", filename, "path", path, "bytes", n)
		return err
	}

	var write func(string, io.WriterTo) error
	if s.readonly {
		write = nil // disable uploads
	} else {
		write = func(filename string, wt io.WriterTo) error {
			path := s.safeJoin(filename)
			s.debug("TFTP write request", "filename", filename, "path", path)
			// O_EXCL prevents overwriting boot loaders accidentally.
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
			if err != nil {
				s.error("Failed to open TFTP file for writing", "filename", filename, "path", path, "err", err)
				return err
			}
			defer func() { _ = f.Close() }()
			n, err := wt.WriteTo(f)
			if err != nil {
				s.error("TFTP write transfer failed", "filename", filename, "path", path, "bytes", n, "err", err)
				return err
			}
			s.debug("TFTP write transfer completed", "filename", filename, "path", path, "bytes", n)
			return err
		}
	}

	core := tftp.NewServer(read, write)
	if s.timeout > 0 {
		core.SetTimeout(s.timeout)
	}
	s.core = core
	return s
}

// WithLogger attaches a logger to server callbacks and transfer hooks.
func (s *Server) WithLogger(logger log.Logger) *Server {
	s.logger = logger
	if s.core != nil {
		s.core.SetHook(tftpHook{logger: logger})
	}
	return s
}

func (s *Server) ListenAndServe() error {
	if s.root == "" {
		return errors.New("tftp: root directory is empty")
	}
	s.info("Starting TFTP server", "addr", s.addr, "root", s.root, "readonly", s.readonly, "timeout", s.timeout)
	return s.core.ListenAndServe(s.addr) // blocks until Shutdown()
}

func (s *Server) Shutdown() { s.core.Shutdown() }

// safeJoin prevents directory traversal outside the TFTP root.
func (s *Server) safeJoin(name string) string {
	clean := filepath.Clean("/" + name)
	clean = strings.TrimPrefix(clean, "/")
	return filepath.Join(s.root, clean)
}

func (s *Server) debug(msg string, args ...any) {
	if s.logger == nil {
		return
	}
	s.logger.Debug(msg, append([]any{"component", "tftp"}, args...)...)
}

func (s *Server) info(msg string, args ...any) {
	if s.logger == nil {
		return
	}
	s.logger.Info(msg, append([]any{"component", "tftp"}, args...)...)
}

func (s *Server) error(msg string, args ...any) {
	if s.logger == nil {
		return
	}
	s.logger.Error(msg, append([]any{"component", "tftp"}, args...)...)
}

type tftpHook struct {
	logger log.Logger
}

func (h tftpHook) OnSuccess(stats tftp.TransferStats) {
	if h.logger == nil {
		return
	}
	h.logger.Info("TFTP transfer succeeded", tftpTransferAttrs(stats)...)
}

func (h tftpHook) OnFailure(stats tftp.TransferStats, err error) {
	if h.logger == nil {
		return
	}
	args := append(tftpTransferAttrs(stats), "err", err)
	h.logger.Warn("TFTP transfer failed", args...)
}

func tftpTransferAttrs(stats tftp.TransferStats) []any {
	return []any{
		"component", "tftp",
		"remote_addr", stats.RemoteAddr.String(),
		"filename", stats.Filename,
		"mode", stats.Mode,
		"duration", stats.Duration,
		"datagrams_sent", stats.DatagramsSent,
		"datagrams_acked", stats.DatagramsAcked,
	}
}
