// Package notebooksdk provides server-side support for Jupyter/Colab integration
// with magic commands, inline visualizations, and interactive feature exploration.
//
// # Usage
//
//	service := notebooksdk.NewService(notebooksdk.DefaultConfig())
//	session, _ := service.CreateSession(notebooksdk.SessionConfig{Notebook: "analysis.ipynb"})
//	result, _ := service.Execute(session.ID, "%feather_get user:123 click_count")
package notebooksdk
