package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/adigunhammedolalekan/luna"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

// FakeDatabase is a simple in-memory database
// just a prove of concept database, all data will
// be lost on app restart
type FakeDatabase struct {
	users map[string]*User // map of users with their Id
}

// NewFakeDatabase creates a new database
func NewFakeDatabase() *FakeDatabase {
	return &FakeDatabase{ users: make(map[string]*User) }
}

// createUser creates a new user with a unique email address
func (db *FakeDatabase) createUser(name, email, password string) (*User, error) {
	// check for duplicate email
	for _, u := range db.users {
		if u.Email == email {
			return nil, errors.New("someone is using that email")
		}
	}

	id := uuid.New().String()
	newUser := &User{Name:name, Id:id, Email:email, Password:password}
	db.users[id] = newUser

	return newUser, nil
}

// getUserByEmail fetch user that owns the supplied
// email address
func (db *FakeDatabase) getUserbyEmail(email string) *User {
	for _, u := range db.users {
		if u.Email == email {
			return u
		}
	}

	return nil
}

// User holds info about a user
type User struct {
	Id string `json:"id"`
	Name string `json:"name"`
	Email string `json:"email"`
	Password string `json:"password"`
}

// WebHandler is the app brain
// featuring a database, websocket handler and
// a pointer to html templates
type WebHandler struct {
	db *FakeDatabase // database
	ws *luna.Luna // luna websocket handler
	templ *template.Template // template renderer
}

// NewWebHandler creates WebHandler
func NewWebHandler(db *FakeDatabase, t *template.Template) *WebHandler {

	// luna extractor function
	// used to assign a unique id to each
	// websocket connection
	extractor := func(r *http.Request) string {
		return r.URL.Query().Get("user")
	}

	// creates Luna
	config := &luna.Config{KeyExtractor:extractor}
	l := luna.New(config)

	return &WebHandler{db:db, ws:l, templ:t}
}

// createUserHandler handles create account request
func (handler *WebHandler) createUserHandler(w http.ResponseWriter, r *http.Request)  {
	name := r.FormValue("name")
	email := r.FormValue("email")
	password := r.FormValue("password")

	// return error if any of the fields is missing
	if name == "" || email == "" || password == "" {
		handler.templ.Lookup("create_account.html").Execute(w, Response{
			Error:true, Message: "required fields are missing",
		})
		return
	}

	// create the user
	newUser, err := handler.db.createUser(name, email, password)
	if err != nil {
		handler.templ.Lookup("create_account.html").Execute(w, Response{
			Error:true, Message: err.Error(),
		})
		return
	}

	// create and set cookie
	cookie := &http.Cookie{
		Value: newUser.Email,
		Expires: time.Now().Add(60000 * time.Minute),
		Name: "ActiveAccount",
		Secure: false,
		Path: "/",
	}
	http.SetCookie(w, cookie)
	http.Redirect(w, r, "/home", http.StatusFound)
}

// loginHandler handles login requests
func (handler *WebHandler) loginHandler(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")

	// returns error if email or password is empty
	if email == "" || password == "" {
		handler.templ.Lookup("sign_in.html").Execute(w, Response{
			Error:true, Message: "required fields are missing",
		})
		return
	}

	// get user or return error if not found
	user := handler.db.getUserbyEmail(email)
	if user == nil {
		handler.templ.Lookup("sign_in.html").Execute(w, Response{
			Error:true, Message: "invalid login details",
		})
		return
	}

	// verify password or return error
	if user.Password != password {
		handler.templ.Lookup("sign_in.html").Execute(w, Response{
			Error:true, Message: "invalid login details",
		})
		return
	}

	// set cookie
	cookie := &http.Cookie{
		Value: user.Email,
		Expires: time.Now().Add(60000 * time.Minute),
		Name: "ActiveAccount",
		Secure: false,
		Path: "/",
	}

	http.SetCookie(w, cookie)
	http.Redirect(w, r, "/home", http.StatusFound)
}

func (handler *WebHandler) renderCreateAccountPage(w http.ResponseWriter, r *http.Request) {
	handler.templ.Lookup("create_account.html").Execute(w, nil)
}

func (handler *WebHandler) renderLoginPage(w http.ResponseWriter, r *http.Request) {
	handler.templ.Lookup("sign_in.html").Execute(w, nil)
}

func (handler *WebHandler) renderHomePage(w http.ResponseWriter, r *http.Request) {
	ck, err := r.Cookie("ActiveAccount")
	if err != nil {
		http.Redirect(w, r, "/authenticate", http.StatusFound)
		return
	}

	user := handler.db.getUserbyEmail(ck.Value)
	if user == nil {
		http.Redirect(w, r, "/authenticate", http.StatusFound)
		return
	}

	handler.templ.Lookup("index.html").Execute(w, Response{
		Error: false, Message: "success", Data: user,
	})
}

func (handler *WebHandler) handleWebSocket(w http.ResponseWriter, r *http.Request)  {
	_ = handler.ws.HandleHttpRequest(w, r)
}

type Message struct {
	Text string `json:"text"`
	From string `json:"from"`
}

func main() {

	// parse templates
	templ := loadViews()

	// create database
	db := NewFakeDatabase()

	// create web handler
	handler := NewWebHandler(db, templ)

	// create router
	router := mux.NewRouter()

	// attach static files server
	router.PathPrefix("/static/").
		Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./ui/static"))))

	router.HandleFunc("/ws/connect", handler.handleWebSocket).Methods("GET")

	router.HandleFunc("/authenticate", handler.renderLoginPage).Methods("GET")
	router.HandleFunc("/authenticate", handler.loginHandler).Methods("POST")

	router.HandleFunc("/user/join", handler.renderCreateAccountPage).Methods("GET")
	router.HandleFunc("/user/join", handler.createUserHandler).Methods("POST")

	router.HandleFunc("/home", handler.renderHomePage)

	// handle websocket messages
	handler.ws.Handle("/user/{id}", func(context *luna.Context) {
		bytes, ok := context.Data . ([]byte)
		if ok {
			var m Message
			if err := json.Unmarshal(bytes, &m); err == nil {
				fmt.Printf("New Message => %s, From %s\n", m.Text, m.From)
			}
		}
	})

	addr := "0.0.0.0:9005"
	log.Printf("Server started at %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal("failed to start http server")
	}
}

// loadViews parse html templates
func loadViews() *template.Template {
	dir := "./ui/templates"
	var allTemplates []string
	data, err := ioutil.ReadDir(dir)

	// quit app if unable to read templates dir
	if err != nil {
		log.Fatal("Failed to read template data ", err)
	}

	// process each template file
	for _, file := range data {
		filename := file.Name()
		if strings.HasSuffix(filename, ".html") {
			allTemplates = append(allTemplates, fmt.Sprintf("%s%s%s", dir, "/", filename))
		}
	}

	// parse templates, quit app in-case an error is encountered
	templates, err := template.ParseFiles(allTemplates...)
	if err != nil {
		log.Fatal("Failed to parse template data ", err)
	}

	return templates
}

// Response
type Response struct {
	Error bool `json:"error"`
	Message string `json:"message"`
	Data interface{} `json:"data"`
}

