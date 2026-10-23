package workspace

import (
	"reflect"
	"testing"

	workspacemodel "agentor/internal/workspace/model"
)

// TestMoveNodeInsertPersists 验证多位置插入保持同级顺序、其他父级数据和当前选择
func TestMoveNodeInsertPersists(t *testing.T) {
	fixture := workspacemodel.State{
		SelectedID: "s2",
		Projects: []workspacemodel.Project{
			{ID: "p1", Tags: []workspacemodel.Tag{{ID: "t1"}, {ID: "t2"}, {ID: "t3"}}, Versions: []workspacemodel.Version{
				{ID: "v1", Features: []workspacemodel.Feature{
					{ID: "f1", Steps: []workspacemodel.Step{{ID: "s1", Prompt: "one"}, {ID: "s2"}, {ID: "s3"}}},
					{ID: "f2", Steps: []workspacemodel.Step{{ID: "s4"}}},
					{ID: "f3"},
				}},
				{ID: "v2", Features: []workspacemodel.Feature{{ID: "f4"}}},
				{ID: "v3"},
			}},
			{ID: "p2", Versions: []workspacemodel.Version{{ID: "v4"}}},
			{ID: "p3"},
		},
	}
	tests := []struct {
		kind    string
		prefix  string
		reorder func(*workspacemodel.State)
	}{
		{"project", "p", func(s *workspacemodel.State) {
			s.Projects = []workspacemodel.Project{s.Projects[1], s.Projects[2], s.Projects[0]}
		}},
		{"version", "v", func(s *workspacemodel.State) {
			items := s.Projects[0].Versions
			s.Projects[0].Versions = []workspacemodel.Version{items[1], items[2], items[0]}
		}},
		{"feature", "f", func(s *workspacemodel.State) {
			items := s.Projects[0].Versions[0].Features
			s.Projects[0].Versions[0].Features = []workspacemodel.Feature{items[1], items[2], items[0]}
		}},
		{"step", "s", func(s *workspacemodel.State) {
			items := s.Projects[0].Versions[0].Features[0].Steps
			s.Projects[0].Versions[0].Features[0].Steps = []workspacemodel.Step{items[1], items[2], items[0]}
		}},
		{"tag", "t", func(s *workspacemodel.State) {
			items := s.Projects[0].Tags
			s.Projects[0].Tags = []workspacemodel.Tag{items[1], items[2], items[0]}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			root := t.TempDir()
			s := &Service{root: root, state: clone(fixture)}
			expected := clone(fixture)
			tt.reorder(&expected)
			got, err := s.MoveNode(tt.kind, tt.prefix+"1", 2)
			if err != nil || !reflect.DeepEqual(got, expected) {
				t.Fatalf("向后插入结果不正确：%#v，%v", got, err)
			}
			reopened, err := NewService(root)
			if err != nil || !reflect.DeepEqual(reopened.GetState(), expected) {
				t.Fatal("排序结果没有正确持久化", err)
			}
			got, err = reopened.MoveNode(tt.kind, tt.prefix+"1", -2)
			if err != nil || !reflect.DeepEqual(got, fixture) {
				t.Fatalf("向前插入未恢复原顺序：%#v，%v", got, err)
			}
			got, err = reopened.MoveNode(tt.kind, tt.prefix+"1", 3)
			if err != nil || !reflect.DeepEqual(got, fixture) {
				t.Fatal("越界移动改变了数据", err)
			}
			if _, err := reopened.MoveNode(tt.kind, "missing", 1); err == nil {
				t.Fatal("不存在的对象被允许排序")
			}
			if _, err := reopened.MoveNode(tt.kind, tt.prefix+"1", 0); err == nil {
				t.Fatal("零距离移动应被拒绝")
			}
		})
	}
}
