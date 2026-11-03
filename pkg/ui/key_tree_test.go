package ui

import (
	"reflect"
	"testing"
)

var treeKeys = []string{
	"cache:home",
	"config",
	"user:1:profile",
	"user:1:session",
	"user:2:profile",
	"user:admin",
}

func TestBuildNodesAtRoot(t *testing.T) {
	got := buildNodes(treeKeys, nil)
	want := []treeNode{
		{kind: nodeDir, name: "cache", prefix: "cache:", count: 1},
		{kind: nodeDir, name: "user", prefix: "user:", count: 4},
		{kind: nodeLeaf, name: "config", fullKey: "config"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildNodes(root):\n got  %+v\n want %+v", got, want)
	}
}

func TestBuildNodesInsideDir(t *testing.T) {
	got := buildNodes(treeKeys, []string{"user"})
	want := []treeNode{
		{kind: nodeDir, name: "1", prefix: "user:1:", count: 2},
		{kind: nodeDir, name: "2", prefix: "user:2:", count: 1},
		{kind: nodeLeaf, name: "admin", fullKey: "user:admin"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildNodes(user):\n got  %+v\n want %+v", got, want)
	}

	got = buildNodes(treeKeys, []string{"user", "1"})
	want = []treeNode{
		{kind: nodeLeaf, name: "profile", fullKey: "user:1:profile"},
		{kind: nodeLeaf, name: "session", fullKey: "user:1:session"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildNodes(user:1):\n got  %+v\n want %+v", got, want)
	}
}

func TestBuildNodesEdgeCases(t *testing.T) {
	t.Run("no keys", func(t *testing.T) {
		if got := buildNodes(nil, nil); len(got) != 0 {
			t.Errorf("got %+v, want none", got)
		}
	})

	t.Run("path with no matching keys", func(t *testing.T) {
		if got := buildNodes(treeKeys, []string{"missing"}); len(got) != 0 {
			t.Errorf("got %+v, want none", got)
		}
	})

	t.Run("prefix must match a whole segment", func(t *testing.T) {
		// "users:9" is not inside "user".
		got := buildNodes([]string{"user:1", "users:9"}, []string{"user"})
		want := []treeNode{{kind: nodeLeaf, name: "1", fullKey: "user:1"}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("empty segments", func(t *testing.T) {
		// "a::b" has an empty middle segment; "a:" ends with the delimiter.
		got := buildNodes([]string{"a::b", "a:"}, []string{"a"})
		want := []treeNode{
			{kind: nodeDir, name: "", prefix: "a::", count: 1},
			{kind: nodeLeaf, name: "", fullKey: "a:"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("korean segments", func(t *testing.T) {
		got := buildNodes([]string{"사용자:1", "사용자:2"}, nil)
		want := []treeNode{{kind: nodeDir, name: "사용자", prefix: "사용자:", count: 2}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})
}

func TestKeysWithPrefix(t *testing.T) {
	got := keysWithPrefix(treeKeys, "user:1:")
	want := []string{"user:1:profile", "user:1:session"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("keysWithPrefix = %v, want %v", got, want)
	}
	if got := keysWithPrefix(treeKeys, "nope:"); got != nil {
		t.Errorf("keysWithPrefix(nope:) = %v, want nil", got)
	}
}

func TestParentPathDoesNotAlias(t *testing.T) {
	segs := []string{"a", "b", "c"}
	parent := parentPath(segs)
	if !reflect.DeepEqual(parent, []string{"a", "b"}) {
		t.Fatalf("parentPath = %v, want [a b]", parent)
	}
	// Appending to the parent must not overwrite the caller's slice.
	_ = append(parent, "x")
	if segs[2] != "c" {
		t.Errorf("parentPath shares its backing array: segs = %v", segs)
	}

	if got := parentPath(nil); got != nil {
		t.Errorf("parentPath(nil) = %v, want nil", got)
	}
}

func TestBreadcrumbString(t *testing.T) {
	tests := []struct {
		segs []string
		want string
	}{
		{nil, "/"},
		{[]string{"user"}, "/user/"},
		{[]string{"user", "1"}, "/user/1/"},
	}
	for _, tt := range tests {
		if got := breadcrumbString(tt.segs); got != tt.want {
			t.Errorf("breadcrumbString(%v) = %q, want %q", tt.segs, got, tt.want)
		}
	}
}
