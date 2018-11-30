package handlers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/isard-vdi/isard-ipxe/pkg/client/list"
	"github.com/isard-vdi/isard-ipxe/pkg/client/request"
	"github.com/isard-vdi/isard-ipxe/pkg/handlers"
)

func TestLoginHandler(t *testing.T) {
	t.Run("should work as expected", func(t *testing.T) {
		r, err := http.NewRequest("GET", "/pxe/boot/login", nil)
		if err != nil {
			t.Fatalf("error preparing the test: %v", err)
		}

		w := httptest.NewRecorder()

		expected := []byte(`#!ipxe
set username
set password
login
chain https://isard.domain.com/pxe/boot/auth?usr=${username:uristring}&pwd=${password:uristring}`)

		handlers.LoginHandler(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expecting %d, but got %d", http.StatusOK, w.Code)
		}

		if !bytes.Equal(w.Body.Bytes(), expected) {
			t.Errorf("expecting %s, but got %s", expected, w.Body.Bytes())
		}

		if err := os.Remove("config.yml"); err != nil {
			t.Fatalf("error finishing the test: %v", err)
		}
	})

	t.Run("should return an error menu", func(t *testing.T) {
		initialFolder, err := os.Getwd()
		if err != nil {
			t.Fatalf("error preparing the test %v", err)
		}

		err = os.Chdir("/")
		if err != nil {
			t.Fatalf("error preparing the test %v", err)
		}

		r, err := http.NewRequest("GET", "/pxe/boot/login", nil)
		if err != nil {
			t.Fatalf("error preparing the test: %v", err)
		}

		w := httptest.NewRecorder()

		expected := []byte(`#!ipxe
echo There was an error reading the configuration file. If this error persists, contact your IsardVDI administrator.
prompt Press any key to try again
reboot`)

		handlers.LoginHandler(w, r)

		// Code needs to be 200, since iPXE doesn't boot 500's
		if w.Code != http.StatusOK {
			t.Errorf("expecting %d, but got %d", http.StatusOK, w.Code)
		}

		if !bytes.Equal(w.Body.Bytes(), expected) {
			t.Errorf("expecting %s, but got %s", expected, w.Body.Bytes())
		}

		if err := os.Chdir(initialFolder); err != nil {
			t.Fatalf("error finishing the test: %v", err)
		}
	})
}

type testWebRequestAuth struct{}

func (testWebRequestAuth) Get(url string) ([]byte, int, error) {
	return nil, 500, nil
}

func (testWebRequestAuth) Post(url string, body io.Reader) ([]byte, int, error) {
	type rsp struct {
		body []byte
		code int
		err  error
	}

	type authBody struct {
		Username string `json:"usr"`
		Password string `json:"pwd"`
	}

	endpoints := []struct {
		url  string
		body authBody
		rsp  rsp
	}{
		{
			url: "https://isard.domain.com/pxe/login",
			body: authBody{
				Username: "nefix",
				Password: "P4$$w0rd! ",
			},
			rsp: rsp{
				body: []byte(`{
	"tkn": "cr7B-duhaj3YkMIAmv1jZOb_ytH-23ruSnKwlVHWxrU"
}`),
				code: 200,
				err:  nil,
			},
		},
		{
			url: "https://isard.domain.com/pxe/login",
			body: authBody{
				Username: "nefix",
				Password: "invalidpassword",
			},
			rsp: rsp{
				body: []byte(""),
				code: 401,
				err:  nil,
			},
		},
		{
			url: "https://isard.domain.com/pxe/login",
			body: authBody{
				Username: "nefix",
				Password: "error",
			},
			rsp: rsp{
				body: []byte(""),
				code: 500,
				err:  nil,
			},
		},
	}

	bodyBuf := new(bytes.Buffer)
	bodyBuf.ReadFrom(body)

	var decodedBuffer authBody

	err := json.Unmarshal(bodyBuf.Bytes(), &decodedBuffer)
	if err != nil {
		return nil, 500, fmt.Errorf("bad formatted body: %v", err)
	}

	for _, endpoint := range endpoints {
		if url == endpoint.url {
			if reflect.DeepEqual(decodedBuffer, endpoint.body) {
				return endpoint.rsp.body, endpoint.rsp.code, endpoint.rsp.err
			}
		}
	}

	return []byte("The endpoint wasn't found!"), 404, nil
}

func TestAuthHandler(t *testing.T) {
	t.Run("should work as expected", func(t *testing.T) {
		handlers.WebRequest = testWebRequestAuth{}

		r, err := http.NewRequest("GET", "/pxe/boot/auth?usr=nefix&pwd=P4$$w0rd! ", nil)
		if err != nil {
			t.Fatalf("error preparing the test: %v", err)
		}

		w := httptest.NewRecorder()

		expected := []byte(`#!ipxe
chain https://isard.domain.com/pxe/boot/list?tkn=cr7B-duhaj3YkMIAmv1jZOb_ytH-23ruSnKwlVHWxrU&usr=nefix`)

		handlers.AuthHandler(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expecting %d, but got %d", http.StatusOK, w.Code)
		}

		if !bytes.Equal(w.Body.Bytes(), expected) {
			t.Errorf("expecting %s, but got %s", expected, w.Body.Bytes())
		}

		if err := os.Remove("config.yml"); err != nil {
			t.Fatalf("error finishing the test: %v", err)
		}
	})

	t.Run("should return a login menu", func(t *testing.T) {
		handlers.WebRequest = testWebRequestAuth{}

		r, err := http.NewRequest("GET", "/pxe/boot/auth?usr=nefix&pwd=invalidpassword", nil)
		if err != nil {
			t.Fatalf("error preparing the test: %v", err)
		}

		w := httptest.NewRecorder()

		expected := []byte(`#!ipxe
set username
set password
login
chain https://isard.domain.com/pxe/boot/auth?usr=${username:uristring}&pwd=${password:uristring}`)

		handlers.AuthHandler(w, r)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expecting %d, but got %d", http.StatusUnauthorized, w.Code)
		}

		if !bytes.Equal(w.Body.Bytes(), expected) {
			t.Errorf("expecting %s, but got %s", expected, w.Body.Bytes())
		}

		if err := os.Remove("config.yml"); err != nil {
			t.Fatalf("error finishing the test: %v", err)
		}
	})

	t.Run("should return an error menu", func(t *testing.T) {
		handlers.WebRequest = testWebRequestAuth{}

		r, err := http.NewRequest("GET", "/pxe/boot/auth?usr=nefix&pwd=error", nil)
		if err != nil {
			t.Fatalf("error preparing the test: %v", err)
		}

		w := httptest.NewRecorder()

		expected := []byte(`#!ipxe
echo There was an error calling the login API endpoint. If this error persists, contact your IsardVDI administrator.
prompt Press any key to try again
reboot`)

		handlers.AuthHandler(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expecting %d, but got %d", http.StatusOK, w.Code)
		}

		if !bytes.Equal(w.Body.Bytes(), expected) {
			t.Errorf("expecting %s, but got %s", expected, w.Body.Bytes())
		}

		if err := os.Remove("config.yml"); err != nil {
			t.Fatalf("error finishing the test: %v", err)
		}
	})
}

type testWebRequestList struct{}

func (testWebRequestList) Get(url string) ([]byte, int, error) {
	return endpointsList[url].Body, endpointsList[url].Code, endpointsList[url].Err
}

func (testWebRequestList) Post(url string, body io.Reader) ([]byte, int, error) {
	return nil, 500, nil
}

var jsonEmptyListList, _ = json.Marshal(&list.VMList{})

var endpointsList = map[string]endpointKeyList{
	"https://isard.domain.com/pxe/list?tkn=ShibAWD6OKjA8950vRIPUEZu848Ke0Rzp3Oxtye_V1c": {
		Body: []byte(`{
					"vms": [
						{
							"id": "_nefix_KDE_Neon_5",
							"name": "KDE Neon 5",
							"description": "This is a VM that's using KDE Neon 5"
						},
						{
							"id": "_nefix_Debian_9",
							"name": "Debian 9",
							"description": "This is a VM that's using Debian 9"
						},
						{
							"id": "_nefix_Arch_Linux",
							"name": "Arch Linux",
							"description": "This is a VM that's using Arch Linux"
						}
					]
				}`),
		Code: 200,
		Err:  nil,
	},
	"https://isard.domain.com/pxe/list?tkn=invalidtoken": {
		Body: jsonEmptyListList,
		Code: 403,
		Err:  nil,
	},
}

type endpointKeyList struct {
	Body []byte
	Code int
	Err  error
}

func TestVMListHandler(t *testing.T) {
	t.Run("should work as expected", func(t *testing.T) {
		handlers.WebRequest = testWebRequestList{}

		r, err := http.NewRequest("GET", "/pxe/boot/list?tkn=ShibAWD6OKjA8950vRIPUEZu848Ke0Rzp3Oxtye_V1c", nil)
		if err != nil {
			t.Fatalf("error preparing the test: %v", err)
		}

		w := httptest.NewRecorder()

		expected := []byte(`#!ipxe
set tkn ShibAWD6OKjA8950vRIPUEZu848Ke0Rzp3Oxtye_V1c
menu IsardVDI - 
item _nefix_KDE_Neon_5 KDE Neon 5 -->
item _nefix_Debian_9 Debian 9 -->
item _nefix_Arch_Linux Arch Linux -->
item
item --gap -- ---- Actions ----
item bootFromDisk Boot from disk -->
item reboot Reboot -->
item poweroff Poweroff -->
choose target && goto ${target}
:_nefix_KDE_Neon_5
chain https://isard.domain.com/pxe/boot/start?tkn=${tkn:uristring}&id=_nefix_KDE_Neon_5
:_nefix_Debian_9
chain https://isard.domain.com/pxe/boot/start?tkn=${tkn:uristring}&id=_nefix_Debian_9
:_nefix_Arch_Linux
chain https://isard.domain.com/pxe/boot/start?tkn=${tkn:uristring}&id=_nefix_Arch_Linux
:bootFromDisk
sanboot --no-describe --drive 0x80
:reboot
reboot
:poweroff
poweroff`)

		handlers.VMListHandler(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expecting %d, but got %d", http.StatusOK, w.Code)
		}

		if !bytes.Equal(w.Body.Bytes(), expected) {
			t.Errorf("expecting %s, but got %s", expected, w.Body.Bytes())
		}

		if err := os.Remove("config.yml"); err != nil {
			t.Fatalf("error finishing the test: %v", err)
		}
	})

	t.Run("should return a login menu", func(t *testing.T) {
		handlers.WebRequest = testWebRequestList{}

		r, err := http.NewRequest("GET", "/pxe/boot/list?tkn=invalidtoken", nil)
		if err != nil {
			t.Fatalf("error preparing the test: %v", err)
		}

		w := httptest.NewRecorder()

		expected := []byte(`#!ipxe
set username
set password
login
chain https://isard.domain.com/pxe/boot/auth?usr=${username:uristring}&pwd=${password:uristring}`)

		handlers.VMListHandler(w, r)

		if w.Code != http.StatusForbidden {
			t.Errorf("expecting %d, but got %d", http.StatusForbidden, w.Code)
		}

		if !bytes.Equal(w.Body.Bytes(), expected) {
			t.Errorf("expecting %s, but got %s", expected, w.Body.Bytes())
		}

		if err := os.Remove("config.yml"); err != nil {
			t.Fatalf("error finishing the test: %v", err)
		}
	})

	t.Run("should return an error menu", func(t *testing.T) {
		handlers.WebRequest = request.Request{}

		r, err := http.NewRequest("GET", "/pxe/boot/list", nil)
		if err != nil {
			t.Fatalf("error preparing the test: %v", err)
		}

		w := httptest.NewRecorder()

		expected := []byte(`#!ipxe
echo There was an error calling the API. If this error persists, contact your IsardVDI administrator.
prompt Press any key to try again
reboot`)

		handlers.VMListHandler(w, r)

		// Code needs to be 200, since iPXE doesn't boot 500's
		if w.Code != http.StatusOK {
			t.Errorf("expecting %d, but got %d", http.StatusOK, w.Code)
		}

		if !bytes.Equal(w.Body.Bytes(), expected) {
			t.Errorf("expecting %s, but got %s", expected, w.Body.Bytes())
		}

		if err := os.Remove("config.yml"); err != nil {
			t.Fatalf("error finishing the test: %v", err)
		}
	})
}
