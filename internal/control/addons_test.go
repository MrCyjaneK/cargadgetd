package control

import "testing"

func TestNewer(t *testing.T) {
	if !newer("1.2.0", "1.1.9") {
		t.Fatal("1.2.0 should beat 1.1.9")
	}
	if newer("1.0", "1.0.1") {
		t.Fatal("1.0 is older than 1.0.1")
	}
	if !newer("b", "a") {
		t.Fatal("b should beat a")
	}
	if newer("1", "1") {
		t.Fatal("equal versions are not newer")
	}
	if !newer("0.2-3", "0.1-1") {
		t.Fatal("0.2-3 should beat 0.1-1")
	}
}

func TestPickRepo(t *testing.T) {
	index := "4.6.0.11/\n5.0.0.55/\n5.1.0.0/\nnot-a-repo\n"
	if got := pickRepo(index, "5.0.0.68"); got != "5.0.0.55" {
		t.Fatalf("device pick %q", got)
	}
	if got := pickRepo(index, ""); got != "5.1.0.0" {
		t.Fatalf("open pick %q", got)
	}
	if got := pickRepo("no versions\n", "5.0.0.55"); got != "" {
		t.Fatalf("empty pick %q", got)
	}
}

func TestBuildRows(t *testing.T) {
	body := []byte(`{
	  "sfos": "5.0.0.55",
	  "recipes": [
	    {
	      "name": "Cargadgetd Plugin Maps",
	      "description": "Maps",
	      "icon": "maps.png",
	      "rpms": {"aarch64": [
	        {"name": "cargadgetd-plugin-maps", "version": "2.0", "release": "1",
	         "provides": ["libcargadgetd_app_maps.so()(64bit)"]}
	      ]}
	    },
	    {
	      "name": "Other",
	      "rpms": {"aarch64": [{"name": "nope", "version": "1", "release": "1"}]}
	    },
	    {
	      "name": "Cargadgetd",
	      "description": "Phone stack\nmore",
	      "icon": "icon.png",
	      "rpms": {"aarch64": [
	        {"name": "cargadgetd-debuginfo", "version": "9", "release": "1"},
	        {"name": "cargadgetd", "version": "0.2", "release": "3",
	         "provides": ["libcargadgetd_app_daemon.so"]}
	      ]}
	    },
	    {
	      "name": "Cargadgetd Shell Basic",
	      "description": "Shell",
	      "rpms": {"aarch64": [
	        {"name": "cargadgetd-shell-basic", "version": "1.0", "release": "1",
	         "provides": ["libcargadgetd_shell_basic.so"]}
	      ]}
	    }
	  ]
	}`)
	rows, err := buildRows(body, installedSet{
		RPMs:  []string{"cargadgetd 0.1-1", "cargadgetd-shell-basic 1.0-1"},
		Files: []string{"libcargadgetd_shell_basic.so"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows %d", len(rows))
	}
	if rows[0].Group != "Cargadgetd" || rows[0].Title != "Cargadgetd" {
		t.Fatalf("app row %+v", rows[0])
	}
	if rows[0].Description != "Phone stack" || !rows[0].Installed || !rows[0].Update {
		t.Fatalf("app flags %+v", rows[0])
	}
	if rows[0].URL != "https://repo-oss.ftuna.eu/status#5.0.0.55/Cargadgetd" {
		t.Fatalf("url %s", rows[0].URL)
	}
	if rows[1].Group != "Shells" || !rows[1].Installed || rows[1].Update {
		t.Fatalf("shell %+v", rows[1])
	}
	if rows[2].Group != "Apps" || rows[2].Installed || rows[2].Title != "Cargadgetd Plugin Maps" {
		t.Fatalf("maps %+v", rows[2])
	}
	if _, err := buildRows([]byte("{"), installedSet{}); err == nil {
		t.Fatal("expected json error")
	}
}

func TestBuildRowsArchOrder(t *testing.T) {
	body := []byte(`{
	  "sfos": "5.0.0.55",
	  "recipes": [{
	    "name": "Cargadgetd Plugin Maps",
	    "rpms": {
	      "i486": [{"name": "cargadgetd-plugin-maps-debuginfo", "version": "1", "release": "1"}],
	      "aarch64": [{"name": "cargadgetd-plugin-maps", "version": "2.0", "release": "4", "provides": ["libcargadgetd_app_maps.so"]}]
	    }
	  }]
	}`)
	rows, err := buildRows(body, installedSet{
		RPMs:  []string{"cargadgetd-plugin-maps 2.0-1"},
		Files: []string{"libcargadgetd_app_maps.so"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || !rows[0].Installed || !rows[0].Update {
		t.Fatalf("%+v", rows)
	}
}
