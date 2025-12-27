package groups

import (
	"testing"
	"time"
)

func TestManager_CreateGroup(t *testing.T) {
	m := NewManager()

	tests := []struct {
		name      string
		group     *FeatureGroup
		createdBy string
		wantErr   error
	}{
		{
			name: "valid group",
			group: &FeatureGroup{
				ID:         "user-features",
				Name:       "User Features",
				EntityType: "user",
				Tags:       []string{"ml", "fraud"},
			},
			createdBy: "admin",
			wantErr:   nil,
		},
		{
			name: "missing ID",
			group: &FeatureGroup{
				Name: "User Features",
			},
			createdBy: "admin",
			wantErr:   ErrGroupIDRequired,
		},
		{
			name: "missing name",
			group: &FeatureGroup{
				ID: "test-group",
			},
			createdBy: "admin",
			wantErr:   ErrGroupNameRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.CreateGroup(tt.group, tt.createdBy)
			if err != tt.wantErr {
				t.Errorf("CreateGroup() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestManager_CreateGroup_Duplicate(t *testing.T) {
	m := NewManager()

	group := &FeatureGroup{
		ID:   "test-group",
		Name: "Test Group",
	}

	err := m.CreateGroup(group, "admin")
	if err != nil {
		t.Fatalf("First CreateGroup() failed: %v", err)
	}

	err = m.CreateGroup(group, "admin")
	if err != ErrGroupExists {
		t.Errorf("CreateGroup() duplicate error = %v, want %v", err, ErrGroupExists)
	}
}

func TestManager_GetGroup(t *testing.T) {
	m := NewManager()

	group := &FeatureGroup{
		ID:          "user-features",
		Name:        "User Features",
		Description: "Features for user entity",
		EntityType:  "user",
	}

	err := m.CreateGroup(group, "admin")
	if err != nil {
		t.Fatalf("CreateGroup() failed: %v", err)
	}

	// Get existing group
	got := m.GetGroup("user-features")
	if got == nil {
		t.Error("GetGroup() returned nil for existing group")
	}
	if got.Name != "User Features" {
		t.Errorf("GetGroup() Name = %v, want %v", got.Name, "User Features")
	}

	// Get non-existing group
	got = m.GetGroup("non-existing")
	if got != nil {
		t.Error("GetGroup() should return nil for non-existing group")
	}
}

func TestManager_UpdateGroup(t *testing.T) {
	m := NewManager()

	group := &FeatureGroup{
		ID:          "user-features",
		Name:        "User Features",
		Description: "Original description",
		EntityType:  "user",
	}

	err := m.CreateGroup(group, "admin")
	if err != nil {
		t.Fatalf("CreateGroup() failed: %v", err)
	}

	// Update the group
	updated := &FeatureGroup{
		ID:          "user-features",
		Name:        "Updated User Features",
		Description: "Updated description",
		EntityType:  "user",
	}

	err = m.UpdateGroup(updated, "editor")
	if err != nil {
		t.Fatalf("UpdateGroup() failed: %v", err)
	}

	got := m.GetGroup("user-features")
	if got.Name != "Updated User Features" {
		t.Errorf("UpdateGroup() Name = %v, want %v", got.Name, "Updated User Features")
	}
	if got.Version != 2 {
		t.Errorf("UpdateGroup() Version = %v, want %v", got.Version, 2)
	}
	if got.UpdatedBy != "editor" {
		t.Errorf("UpdateGroup() UpdatedBy = %v, want %v", got.UpdatedBy, "editor")
	}
}

func TestManager_UpdateGroup_NotFound(t *testing.T) {
	m := NewManager()

	updated := &FeatureGroup{
		ID:   "non-existing",
		Name: "Test",
	}

	err := m.UpdateGroup(updated, "editor")
	if err != ErrGroupNotFound {
		t.Errorf("UpdateGroup() error = %v, want %v", err, ErrGroupNotFound)
	}
}

func TestManager_DeleteGroup(t *testing.T) {
	m := NewManager()

	group := &FeatureGroup{
		ID:   "user-features",
		Name: "User Features",
	}

	err := m.CreateGroup(group, "admin")
	if err != nil {
		t.Fatalf("CreateGroup() failed: %v", err)
	}

	err = m.DeleteGroup("user-features")
	if err != nil {
		t.Fatalf("DeleteGroup() failed: %v", err)
	}

	got := m.GetGroup("user-features")
	if got != nil {
		t.Error("GetGroup() should return nil after deletion")
	}
}

func TestManager_DeleteGroup_NotFound(t *testing.T) {
	m := NewManager()

	err := m.DeleteGroup("non-existing")
	if err != ErrGroupNotFound {
		t.Errorf("DeleteGroup() error = %v, want %v", err, ErrGroupNotFound)
	}
}

func TestManager_SetStatus(t *testing.T) {
	m := NewManager()

	group := &FeatureGroup{
		ID:   "user-features",
		Name: "User Features",
	}

	err := m.CreateGroup(group, "admin")
	if err != nil {
		t.Fatalf("CreateGroup() failed: %v", err)
	}

	err = m.SetStatus("user-features", GroupStatusActive, "admin")
	if err != nil {
		t.Fatalf("SetStatus() failed: %v", err)
	}

	got := m.GetGroup("user-features")
	if got.Status != GroupStatusActive {
		t.Errorf("SetStatus() Status = %v, want %v", got.Status, GroupStatusActive)
	}
}

func TestManager_AddFeature(t *testing.T) {
	m := NewManager()

	group := &FeatureGroup{
		ID:   "user-features",
		Name: "User Features",
	}

	err := m.CreateGroup(group, "admin")
	if err != nil {
		t.Fatalf("CreateGroup() failed: %v", err)
	}

	feature := GroupFeature{
		Name:        "user_age",
		DataType:    "int",
		Description: "Age of the user",
		Required:    true,
	}

	err = m.AddFeature("user-features", feature, "admin")
	if err != nil {
		t.Fatalf("AddFeature() failed: %v", err)
	}

	got := m.GetGroup("user-features")
	if len(got.Features) != 1 {
		t.Errorf("AddFeature() Features count = %v, want %v", len(got.Features), 1)
	}
	if got.Features[0].Name != "user_age" {
		t.Errorf("AddFeature() Feature name = %v, want %v", got.Features[0].Name, "user_age")
	}
}

func TestManager_AddFeature_Duplicate(t *testing.T) {
	m := NewManager()

	group := &FeatureGroup{
		ID:   "user-features",
		Name: "User Features",
	}

	err := m.CreateGroup(group, "admin")
	if err != nil {
		t.Fatalf("CreateGroup() failed: %v", err)
	}

	feature := GroupFeature{
		Name:     "user_age",
		DataType: "int",
	}

	err = m.AddFeature("user-features", feature, "admin")
	if err != nil {
		t.Fatalf("First AddFeature() failed: %v", err)
	}

	err = m.AddFeature("user-features", feature, "admin")
	if err != ErrFeatureExists {
		t.Errorf("AddFeature() duplicate error = %v, want %v", err, ErrFeatureExists)
	}
}

func TestManager_RemoveFeature(t *testing.T) {
	m := NewManager()

	group := &FeatureGroup{
		ID:   "user-features",
		Name: "User Features",
		Features: []GroupFeature{
			{Name: "user_age", DataType: "int"},
			{Name: "user_name", DataType: "string"},
		},
	}

	err := m.CreateGroup(group, "admin")
	if err != nil {
		t.Fatalf("CreateGroup() failed: %v", err)
	}

	err = m.RemoveFeature("user-features", "user_age", "admin")
	if err != nil {
		t.Fatalf("RemoveFeature() failed: %v", err)
	}

	got := m.GetGroup("user-features")
	if len(got.Features) != 1 {
		t.Errorf("RemoveFeature() Features count = %v, want %v", len(got.Features), 1)
	}
	if got.Features[0].Name != "user_name" {
		t.Errorf("RemoveFeature() remaining feature = %v, want %v", got.Features[0].Name, "user_name")
	}
}

func TestManager_RemoveFeature_NotFound(t *testing.T) {
	m := NewManager()

	group := &FeatureGroup{
		ID:   "user-features",
		Name: "User Features",
	}

	err := m.CreateGroup(group, "admin")
	if err != nil {
		t.Fatalf("CreateGroup() failed: %v", err)
	}

	err = m.RemoveFeature("user-features", "non-existing", "admin")
	if err != ErrFeatureNotInGroup {
		t.Errorf("RemoveFeature() error = %v, want %v", err, ErrFeatureNotInGroup)
	}
}

func TestManager_GetFeatureNames(t *testing.T) {
	m := NewManager()

	group := &FeatureGroup{
		ID:   "user-features",
		Name: "User Features",
		Features: []GroupFeature{
			{Name: "user_age", DataType: "int"},
			{Name: "user_name", DataType: "string"},
		},
	}

	err := m.CreateGroup(group, "admin")
	if err != nil {
		t.Fatalf("CreateGroup() failed: %v", err)
	}

	names := m.GetFeatureNames("user-features")
	if len(names) != 2 {
		t.Errorf("GetFeatureNames() count = %v, want %v", len(names), 2)
	}
}

func TestManager_ListGroups(t *testing.T) {
	m := NewManager()

	groups := []*FeatureGroup{
		{ID: "group-1", Name: "Group 1", EntityType: "user", Tags: []string{"ml"}},
		{ID: "group-2", Name: "Group 2", EntityType: "product", Tags: []string{"ml"}},
		{ID: "group-3", Name: "Group 3", EntityType: "user", Owner: "alice"},
	}

	for _, g := range groups {
		err := m.CreateGroup(g, "admin")
		if err != nil {
			t.Fatalf("CreateGroup() failed: %v", err)
		}
	}

	// List all
	all := m.ListGroups(nil)
	if len(all) != 3 {
		t.Errorf("ListGroups() count = %v, want %v", len(all), 3)
	}

	// Filter by entity type
	userGroups := m.ListGroups(&GroupFilter{EntityType: "user"})
	if len(userGroups) != 2 {
		t.Errorf("ListGroups(EntityType=user) count = %v, want %v", len(userGroups), 2)
	}

	// Filter by owner
	aliceGroups := m.ListGroups(&GroupFilter{Owner: "alice"})
	if len(aliceGroups) != 1 {
		t.Errorf("ListGroups(Owner=alice) count = %v, want %v", len(aliceGroups), 1)
	}

	// Filter by tag
	mlGroups := m.ListGroups(&GroupFilter{Tags: []string{"ml"}})
	if len(mlGroups) != 2 {
		t.Errorf("ListGroups(Tags=ml) count = %v, want %v", len(mlGroups), 2)
	}
}

func TestManager_GetGroupsByEntity(t *testing.T) {
	m := NewManager()

	groups := []*FeatureGroup{
		{ID: "group-1", Name: "Group 1", EntityType: "user"},
		{ID: "group-2", Name: "Group 2", EntityType: "user"},
		{ID: "group-3", Name: "Group 3", EntityType: "product"},
	}

	for _, g := range groups {
		err := m.CreateGroup(g, "admin")
		if err != nil {
			t.Fatalf("CreateGroup() failed: %v", err)
		}
	}

	userGroups := m.GetGroupsByEntity("user")
	if len(userGroups) != 2 {
		t.Errorf("GetGroupsByEntity(user) count = %v, want %v", len(userGroups), 2)
	}
}

func TestManager_GetGroupsByTag(t *testing.T) {
	m := NewManager()

	groups := []*FeatureGroup{
		{ID: "group-1", Name: "Group 1", Tags: []string{"ml", "fraud"}},
		{ID: "group-2", Name: "Group 2", Tags: []string{"ml"}},
		{ID: "group-3", Name: "Group 3", Tags: []string{"analytics"}},
	}

	for _, g := range groups {
		err := m.CreateGroup(g, "admin")
		if err != nil {
			t.Fatalf("CreateGroup() failed: %v", err)
		}
	}

	mlGroups := m.GetGroupsByTag("ml")
	if len(mlGroups) != 2 {
		t.Errorf("GetGroupsByTag(ml) count = %v, want %v", len(mlGroups), 2)
	}

	fraudGroups := m.GetGroupsByTag("fraud")
	if len(fraudGroups) != 1 {
		t.Errorf("GetGroupsByTag(fraud) count = %v, want %v", len(fraudGroups), 1)
	}
}

func TestManager_CreateView(t *testing.T) {
	m := NewManager()

	group := &FeatureGroup{
		ID:   "user-features",
		Name: "User Features",
		Features: []GroupFeature{
			{Name: "user_age", DataType: "int"},
			{Name: "user_name", DataType: "string"},
			{Name: "user_email", DataType: "string"},
		},
	}

	err := m.CreateGroup(group, "admin")
	if err != nil {
		t.Fatalf("CreateGroup() failed: %v", err)
	}

	view := &GroupView{
		ID:       "user-basic-view",
		Name:     "Basic User View",
		GroupID:  "user-features",
		Features: []string{"user_age", "user_name"},
	}

	err = m.CreateView(view)
	if err != nil {
		t.Fatalf("CreateView() failed: %v", err)
	}

	got := m.GetView("user-basic-view")
	if got == nil {
		t.Error("GetView() returned nil")
	}
	if len(got.Features) != 2 {
		t.Errorf("CreateView() Features count = %v, want %v", len(got.Features), 2)
	}
}

func TestManager_CreateView_Errors(t *testing.T) {
	m := NewManager()

	group := &FeatureGroup{
		ID:   "user-features",
		Name: "User Features",
	}
	m.CreateGroup(group, "admin")

	tests := []struct {
		name    string
		view    *GroupView
		wantErr error
	}{
		{
			name: "missing view ID",
			view: &GroupView{
				Name:    "Test View",
				GroupID: "user-features",
			},
			wantErr: ErrViewIDRequired,
		},
		{
			name: "missing group ID",
			view: &GroupView{
				ID:   "test-view",
				Name: "Test View",
			},
			wantErr: ErrGroupIDRequired,
		},
		{
			name: "non-existing group",
			view: &GroupView{
				ID:      "test-view",
				Name:    "Test View",
				GroupID: "non-existing",
			},
			wantErr: ErrGroupNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.CreateView(tt.view)
			if err != tt.wantErr {
				t.Errorf("CreateView() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestManager_DeleteView(t *testing.T) {
	m := NewManager()

	group := &FeatureGroup{ID: "group", Name: "Group"}
	m.CreateGroup(group, "admin")

	view := &GroupView{
		ID:      "test-view",
		Name:    "Test View",
		GroupID: "group",
	}
	m.CreateView(view)

	err := m.DeleteView("test-view")
	if err != nil {
		t.Fatalf("DeleteView() failed: %v", err)
	}

	got := m.GetView("test-view")
	if got != nil {
		t.Error("GetView() should return nil after deletion")
	}
}

func TestManager_DeleteView_NotFound(t *testing.T) {
	m := NewManager()

	err := m.DeleteView("non-existing")
	if err != ErrViewNotFound {
		t.Errorf("DeleteView() error = %v, want %v", err, ErrViewNotFound)
	}
}

func TestManager_ListViews(t *testing.T) {
	m := NewManager()

	group1 := &FeatureGroup{ID: "group-1", Name: "Group 1"}
	group2 := &FeatureGroup{ID: "group-2", Name: "Group 2"}
	m.CreateGroup(group1, "admin")
	m.CreateGroup(group2, "admin")

	views := []*GroupView{
		{ID: "view-1", Name: "View 1", GroupID: "group-1"},
		{ID: "view-2", Name: "View 2", GroupID: "group-1"},
		{ID: "view-3", Name: "View 3", GroupID: "group-2"},
	}

	for _, v := range views {
		m.CreateView(v)
	}

	// List all
	all := m.ListViews("")
	if len(all) != 3 {
		t.Errorf("ListViews() count = %v, want %v", len(all), 3)
	}

	// Filter by group
	group1Views := m.ListViews("group-1")
	if len(group1Views) != 2 {
		t.Errorf("ListViews(group-1) count = %v, want %v", len(group1Views), 2)
	}
}

func TestManager_GetViewFeatures(t *testing.T) {
	m := NewManager()

	group := &FeatureGroup{ID: "group", Name: "Group"}
	m.CreateGroup(group, "admin")

	view := &GroupView{
		ID:       "test-view",
		Name:     "Test View",
		GroupID:  "group",
		Features: []string{"feature1", "feature2"},
	}
	m.CreateView(view)

	features := m.GetViewFeatures("test-view")
	if len(features) != 2 {
		t.Errorf("GetViewFeatures() count = %v, want %v", len(features), 2)
	}
}

func TestManager_GetGroupVersion(t *testing.T) {
	m := NewManager()

	group := &FeatureGroup{
		ID:          "user-features",
		Name:        "User Features",
		Description: "Version 1",
	}

	err := m.CreateGroup(group, "admin")
	if err != nil {
		t.Fatalf("CreateGroup() failed: %v", err)
	}

	// Update to create version 2
	updated := &FeatureGroup{
		ID:          "user-features",
		Name:        "User Features",
		Description: "Version 2",
	}
	m.UpdateGroup(updated, "admin")

	// Get version 1
	v1 := m.GetGroupVersion("user-features", 1)
	if v1 == nil {
		t.Error("GetGroupVersion(1) returned nil")
	}
	if v1.Description != "Version 1" {
		t.Errorf("GetGroupVersion(1) Description = %v, want %v", v1.Description, "Version 1")
	}

	// Get version 2 (current)
	v2 := m.GetGroupVersion("user-features", 2)
	if v2 == nil {
		t.Error("GetGroupVersion(2) returned nil")
	}
	if v2.Description != "Version 2" {
		t.Errorf("GetGroupVersion(2) Description = %v, want %v", v2.Description, "Version 2")
	}
}

func TestManager_GetStats(t *testing.T) {
	m := NewManager()

	groups := []*FeatureGroup{
		{
			ID: "group-1", Name: "Group 1", EntityType: "user",
			Features: []GroupFeature{{Name: "f1"}, {Name: "f2"}},
		},
		{
			ID: "group-2", Name: "Group 2", EntityType: "product",
			Features: []GroupFeature{{Name: "f3"}},
		},
	}

	for _, g := range groups {
		m.CreateGroup(g, "admin")
	}

	view := &GroupView{ID: "view-1", Name: "View 1", GroupID: "group-1"}
	m.CreateView(view)

	stats := m.GetStats()
	if stats.TotalGroups != 2 {
		t.Errorf("GetStats() TotalGroups = %v, want %v", stats.TotalGroups, 2)
	}
	if stats.TotalViews != 1 {
		t.Errorf("GetStats() TotalViews = %v, want %v", stats.TotalViews, 1)
	}
	if stats.TotalFeatures != 3 {
		t.Errorf("GetStats() TotalFeatures = %v, want %v", stats.TotalFeatures, 3)
	}
	if stats.ByEntityType["user"] != 1 {
		t.Errorf("GetStats() ByEntityType[user] = %v, want %v", stats.ByEntityType["user"], 1)
	}
}

func TestGroupFilter_Matches(t *testing.T) {
	group := &FeatureGroup{
		ID:         "test-group",
		Name:       "Test Group",
		EntityType: "user",
		Status:     GroupStatusActive,
		Owner:      "alice",
		Team:       "ml-team",
		Tags:       []string{"ml", "fraud"},
	}

	tests := []struct {
		name   string
		filter *GroupFilter
		want   bool
	}{
		{
			name:   "empty filter matches all",
			filter: &GroupFilter{},
			want:   true,
		},
		{
			name:   "matching entity type",
			filter: &GroupFilter{EntityType: "user"},
			want:   true,
		},
		{
			name:   "non-matching entity type",
			filter: &GroupFilter{EntityType: "product"},
			want:   false,
		},
		{
			name:   "matching status",
			filter: &GroupFilter{Status: GroupStatusActive},
			want:   true,
		},
		{
			name:   "non-matching status",
			filter: &GroupFilter{Status: GroupStatusDraft},
			want:   false,
		},
		{
			name:   "matching owner",
			filter: &GroupFilter{Owner: "alice"},
			want:   true,
		},
		{
			name:   "non-matching owner",
			filter: &GroupFilter{Owner: "bob"},
			want:   false,
		},
		{
			name:   "matching team",
			filter: &GroupFilter{Team: "ml-team"},
			want:   true,
		},
		{
			name:   "matching tag",
			filter: &GroupFilter{Tags: []string{"ml"}},
			want:   true,
		},
		{
			name:   "non-matching tag",
			filter: &GroupFilter{Tags: []string{"analytics"}},
			want:   false,
		},
		{
			name:   "multiple matching criteria",
			filter: &GroupFilter{EntityType: "user", Owner: "alice", Tags: []string{"fraud"}},
			want:   true,
		},
		{
			name:   "partial match fails",
			filter: &GroupFilter{EntityType: "user", Owner: "bob"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.Matches(group); got != tt.want {
				t.Errorf("Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestManager_DeleteGroup_WithViews(t *testing.T) {
	m := NewManager()

	group := &FeatureGroup{ID: "group", Name: "Group"}
	m.CreateGroup(group, "admin")

	// Create some views
	m.CreateView(&GroupView{ID: "view-1", Name: "View 1", GroupID: "group"})
	m.CreateView(&GroupView{ID: "view-2", Name: "View 2", GroupID: "group"})

	// Delete group
	err := m.DeleteGroup("group")
	if err != nil {
		t.Fatalf("DeleteGroup() failed: %v", err)
	}

	// Views should also be deleted
	if m.GetView("view-1") != nil {
		t.Error("view-1 should be deleted with group")
	}
	if m.GetView("view-2") != nil {
		t.Error("view-2 should be deleted with group")
	}
}

func TestManager_Concurrency(t *testing.T) {
	m := NewManager()

	// Create initial group
	group := &FeatureGroup{
		ID:   "concurrent-group",
		Name: "Concurrent Group",
	}
	m.CreateGroup(group, "admin")

	done := make(chan bool)

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				m.GetGroup("concurrent-group")
				m.ListGroups(nil)
				m.GetStats()
			}
			done <- true
		}()
	}

	// Concurrent writes
	for i := 0; i < 5; i++ {
		go func(i int) {
			for j := 0; j < 20; j++ {
				m.SetStatus("concurrent-group", GroupStatusActive, "admin")
				time.Sleep(time.Microsecond)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 15; i++ {
		<-done
	}
}
