package jobqueue_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/jobqueue"
	"github.com/osbuild/osbuild-composer/internal/store"

	"github.com/google/uuid"
)

func sendHTTP(api *jobqueue.API, method, path, body string) {
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	api.ServeHTTP(resp, req)
}

func testRoute(t *testing.T, api *jobqueue.API, method, path, body string, expectedStatus int, expectedJSON string) {
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	api.ServeHTTP(resp, req)

	if resp.Code != expectedStatus {
		t.Errorf("%s: expected status %v, but got %v", path, expectedStatus, resp.Code)
		return
	}

	replyJSON, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("%s: could not read reponse body: %v", path, err)
		return
	}

	if expectedJSON == "" {
		if len(replyJSON) != 0 {
			t.Errorf("%s: expected no response body, but got:\n%s", path, replyJSON)
		}
		return
	}

	var reply, expected interface{}
	err = json.Unmarshal(replyJSON, &reply)
	if err != nil {
		t.Errorf("%s: %v\n%s", path, err, string(replyJSON))
		return
	}

	if expectedJSON == "*" {
		return
	}

	err = json.Unmarshal([]byte(expectedJSON), &expected)
	if err != nil {
		t.Errorf("%s: expected JSON is invalid: %v", path, err)
		return
	}

	if !reflect.DeepEqual(reply, expected) {
		t.Errorf("%s: reply != expected:\n   reply: %s\nexpected: %s", path, strings.TrimSpace(string(replyJSON)), expectedJSON)
		return
	}
}

func TestBasic(t *testing.T) {
	var cases = []struct {
		Method         string
		Path           string
		Body           string
		ExpectedStatus int
		ExpectedJSON   string
	}{
		// Create job with invalid body
		{"POST", "/job-queue/v1/jobs", ``, http.StatusBadRequest, ``},
		// Update job with invalid ID
		{"PATCH", "/job-queue/v1/jobs/foo", `{"status":"RUNNING"}`, http.StatusBadRequest, ``},
		// Update job that does not exist, with invalid body
		{"PATCH", "/job-queue/v1/jobs/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", ``, http.StatusBadRequest, ``},
		// Update job that does not exist
		{"PATCH", "/job-queue/v1/jobs/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", `{"status":"RUNNING"}`, http.StatusNotFound, ``},
	}

	for _, c := range cases {
		api := jobqueue.New(nil, store.New(nil))

		testRoute(t, api, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON)
	}
}

func TestCreate(t *testing.T) {
	id, _ := uuid.Parse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	store := store.New(nil)
	api := jobqueue.New(nil, store)

	err := store.PushCompose(id, &blueprint.Blueprint{}, "tar")
	if err != nil {
		t.Fatalf("error pushing compose: %v", err)
	}

	testRoute(t, api, "POST", "/job-queue/v1/jobs", `{}`, http.StatusCreated,
		`{"id":"ffffffff-ffff-ffff-ffff-ffffffffffff","pipeline":{"build":{"stages":[{"name":"org.osbuild.dnf","options":{"repos":[{"metalink":"https://mirrors.fedoraproject.org/metalink?repo=fedora-$releasever\u0026arch=$basearch","gpgkey":"-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQINBFturGcBEACv0xBo91V2n0uEC2vh69ywCiSyvUgN/AQH8EZpCVtM7NyjKgKm\nbbY4G3R0M3ir1xXmvUDvK0493/qOiFrjkplvzXFTGpPTi0ypqGgxc5d0ohRA1M75\nL+0AIlXoOgHQ358/c4uO8X0JAA1NYxCkAW1KSJgFJ3RjukrfqSHWthS1d4o8fhHy\nKJKEnirE5hHqB50dafXrBfgZdaOs3C6ppRIePFe2o4vUEapMTCHFw0woQR8Ah4/R\nn7Z9G9Ln+0Cinmy0nbIDiZJ+pgLAXCOWBfDUzcOjDGKvcpoZharA07c0q1/5ojzO\n4F0Fh4g/BUmtrASwHfcIbjHyCSr1j/3Iz883iy07gJY5Yhiuaqmp0o0f9fgHkG53\n2xCU1owmACqaIBNQMukvXRDtB2GJMuKa/asTZDP6R5re+iXs7+s9ohcRRAKGyAyc\nYKIQKcaA+6M8T7/G+TPHZX6HJWqJJiYB+EC2ERblpvq9TPlLguEWcmvjbVc31nyq\nSDoO3ncFWKFmVsbQPTbP+pKUmlLfJwtb5XqxNR5GEXSwVv4I7IqBmJz1MmRafnBZ\ng0FJUtH668GnldO20XbnSVBr820F5SISMXVwCXDXEvGwwiB8Lt8PvqzXnGIFDAu3\nDlQI5sxSqpPVWSyw08ppKT2Tpmy8adiBotLfaCFl2VTHwOae48X2dMPBvQARAQAB\ntDFGZWRvcmEgKDMwKSA8ZmVkb3JhLTMwLXByaW1hcnlAZmVkb3JhcHJvamVjdC5v\ncmc+iQI4BBMBAgAiBQJbbqxnAhsPBgsJCAcDAgYVCAIJCgsEFgIDAQIeAQIXgAAK\nCRDvPBEfz8ZZudTnD/9170LL3nyTVUCFmBjT9wZ4gYnpwtKVPa/pKnxbbS+Bmmac\ng9TrT9pZbqOHrNJLiZ3Zx1Hp+8uxr3Lo6kbYwImLhkOEDrf4aP17HfQ6VYFbQZI8\nf79OFxWJ7si9+3gfzeh9UYFEqOQfzIjLWFyfnas0OnV/P+RMQ1Zr+vPRqO7AR2va\nN9wg+Xl7157dhXPCGYnGMNSoxCbpRs0JNlzvJMuAea5nTTznRaJZtK/xKsqLn51D\nK07k9MHVFXakOH8QtMCUglbwfTfIpO5YRq5imxlWbqsYWVQy1WGJFyW6hWC0+RcJ\nOx5zGtOfi4/dN+xJ+ibnbyvy/il7Qm+vyFhCYqIPyS5m2UVJUuao3eApE38k78/o\n8aQOTnFQZ+U1Sw+6woFTxjqRQBXlQm2+7Bt3bqGATg4sXXWPbmwdL87Ic+mxn/ml\nSMfQux/5k6iAu1kQhwkO2YJn9eII6HIPkW+2m5N1JsUyJQe4cbtZE5Yh3TRA0dm7\n+zoBRfCXkOW4krchbgww/ptVmzMMP7GINJdROrJnsGl5FVeid9qHzV7aZycWSma7\nCxBYB1J8HCbty5NjtD6XMYRrMLxXugvX6Q4NPPH+2NKjzX4SIDejS6JjgrP3KA3O\npMuo7ZHMfveBngv8yP+ZD/1sS6l+dfExvdaJdOdgFCnp4p3gPbw5+Lv70HrMjA==\n=BfZ/\n-----END PGP PUBLIC KEY BLOCK-----\n","checksum":"sha256:9f596e18f585bee30ac41c11fb11a83ed6b11d5b341c1cb56ca4015d7717cb97"}],"packages":["dnf","e2fsprogs","policycoreutils","qemu-img","systemd","grub2-pc","tar"],"releasever":"30","basearch":"x86_64"}}]},"stages":[{"name":"org.osbuild.dnf","options":{"repos":[{"metalink":"https://mirrors.fedoraproject.org/metalink?repo=fedora-$releasever\u0026arch=$basearch","gpgkey":"-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQINBFturGcBEACv0xBo91V2n0uEC2vh69ywCiSyvUgN/AQH8EZpCVtM7NyjKgKm\nbbY4G3R0M3ir1xXmvUDvK0493/qOiFrjkplvzXFTGpPTi0ypqGgxc5d0ohRA1M75\nL+0AIlXoOgHQ358/c4uO8X0JAA1NYxCkAW1KSJgFJ3RjukrfqSHWthS1d4o8fhHy\nKJKEnirE5hHqB50dafXrBfgZdaOs3C6ppRIePFe2o4vUEapMTCHFw0woQR8Ah4/R\nn7Z9G9Ln+0Cinmy0nbIDiZJ+pgLAXCOWBfDUzcOjDGKvcpoZharA07c0q1/5ojzO\n4F0Fh4g/BUmtrASwHfcIbjHyCSr1j/3Iz883iy07gJY5Yhiuaqmp0o0f9fgHkG53\n2xCU1owmACqaIBNQMukvXRDtB2GJMuKa/asTZDP6R5re+iXs7+s9ohcRRAKGyAyc\nYKIQKcaA+6M8T7/G+TPHZX6HJWqJJiYB+EC2ERblpvq9TPlLguEWcmvjbVc31nyq\nSDoO3ncFWKFmVsbQPTbP+pKUmlLfJwtb5XqxNR5GEXSwVv4I7IqBmJz1MmRafnBZ\ng0FJUtH668GnldO20XbnSVBr820F5SISMXVwCXDXEvGwwiB8Lt8PvqzXnGIFDAu3\nDlQI5sxSqpPVWSyw08ppKT2Tpmy8adiBotLfaCFl2VTHwOae48X2dMPBvQARAQAB\ntDFGZWRvcmEgKDMwKSA8ZmVkb3JhLTMwLXByaW1hcnlAZmVkb3JhcHJvamVjdC5v\ncmc+iQI4BBMBAgAiBQJbbqxnAhsPBgsJCAcDAgYVCAIJCgsEFgIDAQIeAQIXgAAK\nCRDvPBEfz8ZZudTnD/9170LL3nyTVUCFmBjT9wZ4gYnpwtKVPa/pKnxbbS+Bmmac\ng9TrT9pZbqOHrNJLiZ3Zx1Hp+8uxr3Lo6kbYwImLhkOEDrf4aP17HfQ6VYFbQZI8\nf79OFxWJ7si9+3gfzeh9UYFEqOQfzIjLWFyfnas0OnV/P+RMQ1Zr+vPRqO7AR2va\nN9wg+Xl7157dhXPCGYnGMNSoxCbpRs0JNlzvJMuAea5nTTznRaJZtK/xKsqLn51D\nK07k9MHVFXakOH8QtMCUglbwfTfIpO5YRq5imxlWbqsYWVQy1WGJFyW6hWC0+RcJ\nOx5zGtOfi4/dN+xJ+ibnbyvy/il7Qm+vyFhCYqIPyS5m2UVJUuao3eApE38k78/o\n8aQOTnFQZ+U1Sw+6woFTxjqRQBXlQm2+7Bt3bqGATg4sXXWPbmwdL87Ic+mxn/ml\nSMfQux/5k6iAu1kQhwkO2YJn9eII6HIPkW+2m5N1JsUyJQe4cbtZE5Yh3TRA0dm7\n+zoBRfCXkOW4krchbgww/ptVmzMMP7GINJdROrJnsGl5FVeid9qHzV7aZycWSma7\nCxBYB1J8HCbty5NjtD6XMYRrMLxXugvX6Q4NPPH+2NKjzX4SIDejS6JjgrP3KA3O\npMuo7ZHMfveBngv8yP+ZD/1sS6l+dfExvdaJdOdgFCnp4p3gPbw5+Lv70HrMjA==\n=BfZ/\n-----END PGP PUBLIC KEY BLOCK-----\n","checksum":"sha256:9f596e18f585bee30ac41c11fb11a83ed6b11d5b341c1cb56ca4015d7717cb97"}],"packages":["policycoreutils","selinux-policy-targeted","kernel","firewalld","chrony","langpacks-en"],"exclude_packages":["dracut-config-rescue"],"releasever":"30","basearch":"x86_64"}},{"name":"org.osbuild.fix-bls","options":{}},{"name":"org.osbuild.locale","options":{"language":"en_US"}},{"name":"org.osbuild.grub2","options":{"root_fs_uuid":"76a22bf4-f153-4541-b6c7-0332c0dfaeac","boot_fs_uuid":"00000000-0000-0000-0000-000000000000","kernel_opts":"ro biosdevname=0 net.ifnames=0"}},{"name":"org.osbuild.selinux","options":{"file_contexts":"etc/selinux/targeted/contexts/files/file_contexts"}}],"assembler":{"name":"org.osbuild.tar","options":{"filename":"root.tar.xz"}}},"targets":[{"name":"org.osbuild.local","options":{"location":"/var/lib/osbuild-composer/outputs/ffffffff-ffff-ffff-ffff-ffffffffffff"}}]}`)
}

func testUpdateTransition(t *testing.T, from, to string, expectedStatus int) {
	id, _ := uuid.Parse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	store := store.New(nil)
	api := jobqueue.New(nil, store)

	if from != "VOID" {
		err := store.PushCompose(id, &blueprint.Blueprint{}, "tar")
		if err != nil {
			t.Fatalf("error pushing compose: %v", err)
		}
		if from != "WAITING" {
			sendHTTP(api, "POST", "/job-queue/v1/jobs", `{}`)
			if from != "RUNNING" {
				sendHTTP(api, "PATCH", "/job-queue/v1/jobs/ffffffff-ffff-ffff-ffff-ffffffffffff", `{"status":"`+from+`"}`)
			}
		}
	}

	testRoute(t, api, "PATCH", "/job-queue/v1/jobs/ffffffff-ffff-ffff-ffff-ffffffffffff", `{"status":"`+to+`"}`, expectedStatus, ``)
}

func TestUpdate(t *testing.T) {
	var cases = []struct {
		From           string
		To             string
		ExpectedStatus int
	}{
		{"VOID", "WAITING", http.StatusNotFound},
		{"VOID", "RUNNING", http.StatusNotFound},
		{"VOID", "FINISHED", http.StatusNotFound},
		{"VOID", "FAILED", http.StatusNotFound},
		{"WAITING", "WAITING", http.StatusNotFound},
		{"WAITING", "RUNNING", http.StatusNotFound},
		{"WAITING", "FINISHED", http.StatusNotFound},
		{"WAITING", "FAILED", http.StatusNotFound},
		{"RUNNING", "WAITING", http.StatusBadRequest},
		{"RUNNING", "RUNNING", http.StatusOK},
		{"RUNNING", "FINISHED", http.StatusOK},
		{"RUNNING", "FAILED", http.StatusOK},
		{"FINISHED", "WAITING", http.StatusBadRequest},
		{"FINISHED", "RUNNING", http.StatusBadRequest},
		{"FINISHED", "FINISHED", http.StatusBadRequest},
		{"FINISHED", "FAILED", http.StatusBadRequest},
		{"FAILED", "WAITING", http.StatusBadRequest},
		{"FAILED", "RUNNING", http.StatusBadRequest},
		{"FAILED", "FINISHED", http.StatusBadRequest},
		{"FAILED", "FAILED", http.StatusBadRequest},
	}

	for _, c := range cases {
		testUpdateTransition(t, c.From, c.To, c.ExpectedStatus)
	}
}
