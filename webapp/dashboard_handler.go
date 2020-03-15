package webapp

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator"

	"github.com/peppinux/dero-merchant/auth"
	"github.com/peppinux/dero-merchant/httperror"
	"github.com/peppinux/dero-merchant/webapp/store"
)

// DashboardHandler handles GET requests to /dashboard
func DashboardHandler(c *gin.Context) {
	c.Redirect(http.StatusFound, "/dashboard/stores")
}

type myAccountData struct {
	UserSignedIn bool
	Stores       map[int]string
	Username     string
	Email        string
}

// MyAccountHandler handles GET requests to /dashboard/account
func MyAccountHandler(c *gin.Context) {
	s := c.MustGet("session").(*auth.Session)

	username, err := s.Username()
	if httperror.Render500IfErr(c, err, "Error getting username") != nil {
		return
	}

	email, err := s.Email()
	if httperror.Render500IfErr(c, err, "Error getting email") != nil {
		return
	}

	storesMap, err := s.StoresMap()
	if httperror.Render500IfErr(c, err, "Error getting stores map") != nil {
		return
	}

	resp := &myAccountData{
		UserSignedIn: s.SignedIn,
		Stores:       storesMap,
		Username:     username,
		Email:        email,
	}

	c.HTML(http.StatusOK, "account.html", resp)
}

type myStoresData struct {
	UserSignedIn bool
	Stores       map[int]string
}

// MyStoresHandler handles GET requests to /dashboard/stores
func MyStoresHandler(c *gin.Context) {
	s := c.MustGet("session").(*auth.Session)

	storesMap, err := s.StoresMap()
	if httperror.Render500IfErr(c, err, "Error getting stores map") != nil {
		return
	}

	resp := &myStoresData{
		UserSignedIn: s.SignedIn,
		Stores:       storesMap,
	}

	c.HTML(http.StatusOK, "stores.html", resp)
}

type viewStoreData struct {
	UserSignedIn bool
	Stores       map[int]string
	Store        *store.Store
}

// ViewStoreHandler handles GET requests to /dashboard/stores/view/:id
func ViewStoreHandler(c *gin.Context) {
	// Get Store ID from URL Params
	storeID, err := strconv.Atoi(c.Param("id"))
	if httperror.Render500IfErr(c, err, "Error converting string to int") != nil {
		return
	}

	// Get User data from session
	s := c.MustGet("session").(*auth.Session)
	storesMap, _ := s.StoresMap()

	store, errCode, err := store.FetchStoreFromID(storeID, s.UserID)
	if err != nil {
		if errCode == http.StatusNotFound {
			// If no Store was found because of wrong ID/wrong owner/store removed, redirect to Dashboard
			c.Redirect(http.StatusSeeOther, "/dashboard/stores")
		} else {
			httperror.Render500(c, err, "Error fetching store from ID")
		}
		return
	}

	resp := &viewStoreData{
		UserSignedIn: s.SignedIn,
		Stores:       storesMap,
		Store:        store,
	}

	c.HTML(http.StatusOK, "store.html", resp)
}

type addStoreFields struct {
	Title   string `form:"title" binding:"required,max=32"`
	ViewKey string `form:"viewkey" binding:"required,len=128"`
	Webhook string `form:"webhook"`
}

type addStoreErrors struct {
	Title       bool
	UniqueTitle bool
	ViewKey     bool
}

type addStoreData struct {
	UserSignedIn bool
	Stores       map[int]string
	Fields       *addStoreFields
	Errors       *addStoreErrors
}

// AddStoreGetHandler handles GET requests to /dashboard/stores/add
func AddStoreGetHandler(c *gin.Context) {
	s := c.MustGet("session").(*auth.Session)
	storesMap, _ := s.StoresMap()
	resp := &addStoreData{
		UserSignedIn: s.SignedIn,
		Stores:       storesMap,
		Fields:       &addStoreFields{},
		Errors:       &addStoreErrors{},
	}
	c.HTML(http.StatusOK, "add_store.html", resp)
}

// AddStorePostHandler handles POST requests to /dashboard/stores/add
func AddStorePostHandler(c *gin.Context) {
	// Get User data from session
	sess := c.MustGet("session").(*auth.Session)
	storesMap, _ := sess.StoresMap()

	// Get new Store data submitted by user through form
	var (
		f    addStoreFields
		e    addStoreErrors
		resp = &addStoreData{
			UserSignedIn: sess.SignedIn,
			Stores:       storesMap,
			Fields:       &f,
			Errors:       &e,
		}
	)
	err := c.ShouldBind(&f)
	if err != nil {
		// Validate User input
		errs := err.(validator.ValidationErrors)
		for _, err := range errs {
			switch err.Field() {
			case "Title":
				e.Title = true
			case "ViewKey":
				e.ViewKey = true
			}
		}
		if errs != nil {
			c.HTML(http.StatusUnprocessableEntity, "add_store.html", resp)
			return
		}
	}

	s, errs := store.CreateNewStore(f.Title, f.ViewKey, f.Webhook, sess.UserID)
	if errs != nil {
		for _, err := range errs {
			switch err {
			case store.ErrInvalidTitle:
				e.Title = true
			case store.ErrInvalidViewKey:
				e.ViewKey = true
			case store.ErrTitleNotUnique:
				e.UniqueTitle = true
				c.HTML(http.StatusForbidden, "add_store.html", resp)
				return
			default:
				httperror.Render500(c, err, "Error creating new store")
				return
			}
		}

		if e.Title || e.ViewKey {
			c.HTML(http.StatusUnprocessableEntity, "add_store.html", resp)
			return
		}
	}

	err = s.Insert()
	if httperror.Render500IfErr(c, err, "Error inserting store into DB") != nil {
		return
	}

	// Redirect user to new Store page
	storeURL := fmt.Sprintf("/dashboard/stores/view/%d", s.ID)
	c.Redirect(http.StatusSeeOther, storeURL)
}

type viewStorePaymentsData struct {
	UserSignedIn bool
	Stores       map[int]string
	StoreID      int
}

// ViewStorePaymentsHandler handles GET requests to /dashboard/stores/view/:id/payments
func ViewStorePaymentsHandler(c *gin.Context) {
	// Get Store ID from URL Params
	storeID, err := strconv.Atoi(c.Param("id"))
	if httperror.Render500IfErr(c, err, "Error converting string to int") != nil {
		return
	}

	// Get User data from session
	s := c.MustGet("session").(*auth.Session)
	storesMap, _ := s.StoresMap()

	resp := &viewStorePaymentsData{
		UserSignedIn: s.SignedIn,
		Stores:       storesMap,
		StoreID:      storeID,
	}

	c.HTML(http.StatusOK, "payments.html", resp)
}
