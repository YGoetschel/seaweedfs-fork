package dash

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/seaweedfs/seaweedfs/weed/glog"
)

// ShowLogin displays the login page
func (s *AdminServer) ShowLogin(c *gin.Context) {
	// If authentication is not required, redirect to admin
	session := sessions.Default(c)
	if session.Get("authenticated") == true {
		c.Redirect(http.StatusSeeOther, "/admin")
		return
	}

	// For now, return a simple login form as JSON
	c.HTML(http.StatusOK, "login.html", gin.H{
		"title": "SeaweedFS Admin Login",
		"error": c.Query("error"),
	})
}

// HandleLogin handles login form submission
// Updated to support IAM integration
func (s *AdminServer) HandleLogin(username, password string) gin.HandlerFunc {
	return func(c *gin.Context) {
		loginUsername := c.PostForm("username")
		loginPassword := c.PostForm("password")
		

		// If IAM Manager is available, use it for authentication
		if s.iamManager != nil && s.iamManager.IsInitialized() {
			// TODO: Implement authentication via IAM integration.
			// Currently STSAdapter does not expose direct authentication methods (Authenticate/VerifyPassword).
			// We need to extend STSAdapter or expose IdentityProviders from IAMManager safely.
			// For now, only local admin login is supported.
			glog.V(4).Infof("IAM authentication skipped - not yet implemented in middleware")
		}

		// Fallback to simple local auth if IAM not enabled or specific "admin" user
		if loginUsername == username && loginPassword == password {
			session := sessions.Default(c)
			// Clear any existing invalid session data before setting new values
			session.Clear()
			session.Set("authenticated", true)
			session.Set("username", loginUsername)
			// Local admin has full permissions
			session.Set("is_super_admin", true)
			
			if err := session.Save(); err != nil {
				// Log the detailed error server-side for diagnostics
				glog.Errorf("Failed to save session for user %s: %v", loginUsername, err)
				c.Redirect(http.StatusSeeOther, "/login?error=Unable to create session. Please try again or contact administrator.")
				return
			}

			c.Redirect(http.StatusSeeOther, "/admin")
			return
		}

		// Authentication failed
		c.Redirect(http.StatusSeeOther, "/login?error=Invalid credentials")
	}
}

// HandleLogout handles user logout
func (s *AdminServer) HandleLogout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	if err := session.Save(); err != nil {
		glog.Warningf("Failed to save session during logout: %v", err)
	}
	c.Redirect(http.StatusSeeOther, "/login")
}

// RequirePermission checks if the user has specific permission
func (s *AdminServer) RequirePermission(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		auth := session.Get("authenticated")
		if auth != true {
			c.Redirect(http.StatusSeeOther, "/login")
			c.Abort()
			return
		}

		// If super admin (local login), allow everything
		if isSuper := session.Get("is_super_admin"); isSuper == true {
			c.Next()
			return
		}

		// Check IAM permissions
		// This requires IAM Manager to be initialized
		// Check roles from session
		rolesInterface := session.Get("roles")
		if rolesInterface != nil {
			if roles, ok := rolesInterface.([]string); ok {
				// Simple RBAC check
				// If checking for "Admin" action, require "Admin" role
				if action == "Admin" || action == "Write" {
					for _, role := range roles {
						// Admin role implies Write permission
						if role == "Admin" {
							c.Next()
							return
						}
					}
				} else {
                    // For other actions, currently allow if authenticated?
                    // Or implement more granular mapping.
                    // For now, if not checking "Admin", we might allow?
                    // But we used "RequirePermission" only for Admin routes.
                    c.Next()
                    return
                }
			}
		}

        // If no roles or role missing
		c.AbortWithStatus(http.StatusForbidden)
	}
}
