package repository

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/pktline"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/server"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func serveUploadPack(ctx context.Context, repoPath string, input io.Reader, output io.Writer, advertise bool) error {
	ep, err := transport.NewEndpoint(repoPath)
	if err != nil {
		return err
	}
	session, err := server.NewServer(localRepositoryLoader{}).NewUploadPackSession(ep, nil)
	if err != nil {
		return err
	}
	defer session.Close()
	refs, err := session.AdvertisedReferencesContext(ctx)
	if err != nil {
		return err
	}
	if advertise {
		if err := refs.Encode(output); err != nil {
			return err
		}
	}
	if input == nil {
		return nil
	}
	req := packp.NewUploadPackRequest()
	if err := req.Decode(input); err != nil {
		return err
	}
	response, err := session.UploadPack(ctx, req)
	if err != nil {
		return err
	}
	defer response.Close()
	return response.Encode(output)
}

func TestRemoteTransportsWithoutExecutables(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	source := t.TempDir()
	r, err := git.PlainInit(source, false)
	if err != nil {
		t.Fatal(err)
	}
	w, err := r.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "remote.txt"), []byte("remote"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Add("remote.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Commit("initial", &git.CommitOptions{Author: &object.Signature{Name: "Test", Email: "test@example.test", When: time.Now()}}); err != nil {
		t.Fatal(err)
	}
	t.Run("HTTPS authentication", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			user, password, ok := req.BasicAuth()
			if !ok || user != "git" || password != "test-token" {
				w.Header().Set("WWW-Authenticate", `Basic realm="repository"`)
				w.WriteHeader(401)
				return
			}
			advertise := strings.HasSuffix(req.URL.Path, "/info/refs")
			if advertise {
				w.Header().Set("Content-Type", "application/x-git-upload-pack-advertisement")
				_ = pktline.NewEncoder(w).EncodeString("# service=git-upload-pack\n")
				_ = pktline.NewEncoder(w).Flush()
			} else {
				w.Header().Set("Content-Type", "application/x-git-upload-pack-result")
			}
			var input io.Reader
			if !advertise {
				input = req.Body
			}
			if err := serveUploadPack(req.Context(), source, input, w, advertise); err != nil {
				t.Errorf("serve HTTPS: %v", err)
			}
		}))
		defer srv.Close()
		// Trust the test CA explicitly, never disable certificate validation.
		cert := filepath.Join(t.TempDir(), "ca.pem")
		if err := os.WriteFile(cert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}), 0600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("SSL_CERT_FILE", cert)
		if err := cloneRepository(context.Background(), ConnectionInput{Password: "test-token"}, srv.URL+"/repo.git", filepath.Join(t.TempDir(), "clone"), true); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("SSH password and known hosts", func(t *testing.T) {
		_, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		signer, err := ssh.NewSignerFromKey(key)
		if err != nil {
			t.Fatal(err)
		}
		config := &ssh.ServerConfig{PasswordCallback: func(_ ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if string(password) != "test-password" {
				return nil, fmt.Errorf("denied")
			}
			return nil, nil
		}}
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		clientKey, err := ssh.NewPublicKey(public)
		if err != nil {
			t.Fatal(err)
		}
		config.PublicKeyCallback = func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if string(key.Marshal()) != string(clientKey.Marshal()) {
				return nil, fmt.Errorf("denied")
			}
			return nil, nil
		}
		config.AddHostKey(signer)
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close()
		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				go func() {
					connection, channels, requests, err := ssh.NewServerConn(conn, config)
					if err != nil {
						conn.Close()
						return
					}
					defer connection.Close()
					go ssh.DiscardRequests(requests)
					for incoming := range channels {
						if incoming.ChannelType() != "session" {
							_ = incoming.Reject(ssh.UnknownChannelType, "unsupported")
							continue
						}
						channel, requests, err := incoming.Accept()
						if err != nil {
							return
						}
						for req := range requests {
							if req.Type != "exec" {
								_ = req.Reply(false, nil)
								continue
							}
							_ = req.Reply(true, nil)
							err := serveUploadPack(context.Background(), source, channel, channel, true)
							status := uint32(0)
							if err != nil {
								status = 1
							}
							_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
							channel.Close()
							break
						}
					}
				}()
			}
		}()
		known := filepath.Join(t.TempDir(), "known_hosts")
		if err := os.WriteFile(known, []byte(knownhosts.Line([]string{listener.Addr().String()}, signer.PublicKey())+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("SSH_KNOWN_HOSTS", known)
		url := "ssh://git@" + listener.Addr().String() + "/repo.git"
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := cloneRepository(ctx, ConnectionInput{Password: "test-password"}, url, filepath.Join(t.TempDir(), "clone"), true); err != nil {
			t.Fatal(err)
		}
		home := t.TempDir()
		t.Setenv("HOME", home)
		if err := os.Mkdir(filepath.Join(home, ".ssh"), 0700); err != nil {
			t.Fatal(err)
		}
		block, err := ssh.MarshalPrivateKey(private, "test")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, ".ssh", "id_ed25519"), pem.EncodeToMemory(block), 0600); err != nil {
			t.Fatal(err)
		}
		if err := cloneRepository(ctx, ConnectionInput{}, url, filepath.Join(t.TempDir(), "key-clone"), true); err != nil {
			t.Fatalf("SSH key authentication: %v", err)
		}
		if err := os.WriteFile(known, nil, 0600); err != nil {
			t.Fatal(err)
		}
		if err := cloneRepository(ctx, ConnectionInput{Password: "test-password"}, url, filepath.Join(t.TempDir(), "untrusted"), true); err == nil {
			t.Fatal("unknown SSH host accepted")
		}
	})
}

func TestSnapshotAndToolsWithoutExecutables(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	ctx := context.Background()
	source := t.TempDir()
	r, err := git.PlainInit(source, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "hello.txt"), []byte("hello world\n"), 0600); err != nil {
		t.Fatal(err)
	}
	w, err := r.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.Add("hello.txt"); err != nil {
		t.Fatal(err)
	}
	hash, err := w.Commit("initial", &git.CommitOptions{Author: &object.Signature{Name: "Test", Email: "test@example.test", When: time.Now()}})
	if err != nil {
		t.Fatal(err)
	}
	in, cleanup, err := Prepare(ctx, ConnectionInput{URL: "file://" + source})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if in.PinnedRevision != hash.String() {
		t.Fatalf("wrong revision: %s", in.PinnedRevision)
	}
	if content, err := File(ctx, in, "hello.txt", 1024); err != nil || content != "hello world\n" {
		t.Fatalf("file %q: %v", content, err)
	}
	if matches, err := Search(ctx, in, "world", 10); err != nil || len(matches) != 1 {
		t.Fatalf("search %v: %v", matches, err)
	}
	if entries, err := Tree(ctx, in, "", 10); err != nil || len(entries) != 1 {
		t.Fatalf("tree %v: %v", entries, err)
	}
	if _, err := Metadata(ctx, in); err != nil {
		t.Fatal(err)
	}
	if _, err := Status(ctx, in); err != nil {
		t.Fatal(err)
	}
}
