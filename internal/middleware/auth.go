package middleware

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the JWT claims structure
type Claims struct {
	UserID uint   `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// Auth returns a middleware that validates JWT tokens
func Auth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from Authorization header
		authHeader := c.GetHeader("Authorization")
		tokenString := ""

		if authHeader == "" {
			// Check query param for download links
			tokenString = c.Query("token")
			if tokenString == "" {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "Authorization header is required",
				})
				return
			}
		} else {
			// Extract token from "Bearer <token>"
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "Invalid authorization header format",
				})
				return
			}
			tokenString = parts[1]
		}

		// Parse and validate token
		claims, err := validateToken(tokenString, jwtSecret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": err.Error(),
			})
			return
		}

		// Store claims in context for handlers to use
		c.Set("userID", claims.UserID)
		c.Set("userEmail", claims.Email)
		c.Set("userRole", claims.Role)
		c.Set("claims", claims)

		c.Next()
	}
}

// validateToken parses and validates a JWT token string
func validateToken(tokenString, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return []byte(secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, errors.New("token has expired")
		}
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}

// GetUserID extracts the user ID from the Gin context
func GetUserID(c *gin.Context) uint {
	userID, exists := c.Get("userID")
	if !exists {
		return 0
	}
	return userID.(uint)
}

// GetUserRole extracts the user role from the Gin context
func GetUserRole(c *gin.Context) string {
	role, exists := c.Get("userRole")
	if !exists {
		return ""
	}
	return role.(string)
}

// IsAdmin checks if the current user is an admin
func IsAdmin(c *gin.Context) bool {
	return GetUserRole(c) == "admin"
}

// IsSeller checks if the current user is a seller
func IsSeller(c *gin.Context) bool {
	return GetUserRole(c) == "seller"
}

// RequireAdmin returns a middleware that requires admin role
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !IsAdmin(c) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "No tienes acceso a esta sección",
			})
			return
		}
		c.Next()
	}
}

// RequireRole returns a middleware that requires specific roles
func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole := GetUserRole(c)
		for _, role := range allowedRoles {
			if userRole == role {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": "No tienes acceso a esta sección",
		})
	}
}

// RequireAdminSellerOrOwner returns a middleware that requires admin, seller or being the owner of the resource
func RequireAdminSellerOrOwner() gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUserRole := GetUserRole(c)
		currentUserID := GetUserID(c)

		// Allow if Admin or Seller
		if currentUserRole == "admin" || currentUserRole == "seller" {
			c.Next()
			return
		}

		// Allow if Owner (resource ID matches current user ID)
		// We check for both "user_id" and "id" as param names
		idParam := c.Param("user_id")
		if idParam == "" {
			idParam = c.Param("id")
		}

		if idParam != "" {
			targetID, err := strconv.ParseUint(idParam, 10, 32)
			if err == nil && uint(targetID) == currentUserID {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": "No tienes acceso a esta información",
		})
	}
}

// RequireAdminOrOwner returns a middleware that requires admin role OR the resource owner (user_id param matches current user).
// Used for routes where only admin or the profile owner may act (e.g. update own profile), not sellers acting on other users.
func RequireAdminOrOwner() gin.HandlerFunc {
	return func(c *gin.Context) {
		if IsAdmin(c) {
			c.Next()
			return
		}
		currentUserID := GetUserID(c)
		idParam := c.Param("user_id")
		if idParam == "" {
			idParam = c.Param("id")
		}
		if idParam != "" {
			targetID, err := strconv.ParseUint(idParam, 10, 32)
			if err == nil && uint(targetID) == currentUserID {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": "No tienes acceso a esta sección",
		})
	}
}
