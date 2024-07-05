package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
)

type Item struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ItemDict = map[int]Item

type JsonDatabase struct {
	JsonFilePath  string
	jsonFileMutex sync.Mutex
	DatabaseMutex sync.Mutex
}

func (j *JsonDatabase) Exists() bool {
	_, err := os.Stat(j.JsonFilePath)
	return err == nil
}

func (j *JsonDatabase) CreateJsonFile() error {
	j.jsonFileMutex.Lock()
	defer j.jsonFileMutex.Unlock()

	file, err := os.Create(j.JsonFilePath)
	if err != nil {
		log.Println("create err", err)
		return err
	}
	defer file.Close()

	return nil
}

func (j *JsonDatabase) ReadJsonFile() (ItemDict, error) {
	j.jsonFileMutex.Lock()
	defer j.jsonFileMutex.Unlock()

	file, err := os.Open(j.JsonFilePath)
	if err != nil {
		log.Println("open err", err)
		return nil, err
	}
	defer file.Close()

	bytedata, err := io.ReadAll(file)
	if err != nil {
		log.Println("read err", err)
		return nil, err
	}

	items := make([]Item, 0)
	itemDict := make(ItemDict)
	if err := json.Unmarshal(bytedata, &items); err != nil {
		log.Println("json unmarshal err", err)
		return nil, err
	}
	for _, item := range items {
		itemDict[item.ID] = item
	}

	return itemDict, nil
}

func (j *JsonDatabase) WriteJsonFile(itemDict *ItemDict) error {
	j.jsonFileMutex.Lock()
	defer j.jsonFileMutex.Unlock()

	items := make([]Item, 0)
	for _, item := range *itemDict {
		items = append(items, item)
	}

	bytes, _ := json.Marshal(items)
	file, err := os.Open(j.JsonFilePath)
	if err != nil {
		log.Println("open err", err)
		return err
	}
	defer file.Close()

	err = os.WriteFile(j.JsonFilePath, bytes, 0644)
	if err != nil {
		log.Println("write err", err)
		return err
	}

	return nil
}

var jsonDatabase = JsonDatabase{
	JsonFilePath:  "./db/data.json",
	jsonFileMutex: sync.Mutex{},
}

func getItem(w http.ResponseWriter, r *http.Request) {
	id, err := extractPathParameters(r)
	if err != nil {
		log.Println("Invalid Parameter", err)
		http.Error(w, "Invalid Parameter", http.StatusBadRequest)
		return
	}

	loadedItems, err := jsonDatabase.ReadJsonFile()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if _, ok := loadedItems[id]; !ok {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(loadedItems[id])
	log.Println("Get item", loadedItems[id])
}

func deleteItem(w http.ResponseWriter, r *http.Request) {
	id, err := extractPathParameters(r)
	if err != nil {
		log.Println("Invalid Parameter", err)
		http.Error(w, "Invalid Parameter", http.StatusBadRequest)
		return
	}

	jsonDatabase.DatabaseMutex.Lock()
	defer jsonDatabase.DatabaseMutex.Unlock()

	loadedItems, err := jsonDatabase.ReadJsonFile()

	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if _, ok := loadedItems[id]; !ok {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	delete(loadedItems, id)
	jsonDatabase.WriteJsonFile(&loadedItems)
	log.Println("Deleted item", loadedItems[id])

	w.WriteHeader(http.StatusNoContent)
}

func postItem(w http.ResponseWriter, r *http.Request) {
	var newItem Item

	if err := json.NewDecoder(r.Body).Decode(&newItem); err != nil {
		http.Error(w, "Invalid Parameter", http.StatusBadRequest)
		return
	}

	jsonDatabase.DatabaseMutex.Lock()
	defer jsonDatabase.DatabaseMutex.Unlock()

	loadedItems, err := jsonDatabase.ReadJsonFile()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	loadedItems[newItem.ID] = newItem
	jsonDatabase.WriteJsonFile(&loadedItems)

	log.Println("Posted item", loadedItems[newItem.ID])
	json.NewEncoder(w).Encode(loadedItems[newItem.ID])
}

func getNthPathSegment(pathSegments *[]string, n int) (string, error) {
	if n < 0 || n >= len(*pathSegments) {
		return "", fmt.Errorf("index %d out of range", n)
	}

	return (*pathSegments)[n], nil
}

func extractPathParameters(r *http.Request) (int, error) {
	pathSegments := strings.Split(r.URL.Path, "/")

	idStr, err := getNthPathSegment(&pathSegments, 2)
	if err != nil {
		return 0, err
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return 0, err
	}

	return id, nil
}

type Route struct {
	Method  string
	Path    string
	Handler http.HandlerFunc
}

func requestRouter(responseWriter http.ResponseWriter, request *http.Request) {
	// GET /{id}
	getItemRoute := Route{
		Method:  http.MethodGet,
		Path:    "/",
		Handler: getItem,
	}
	// POST /
	postItemRoute := Route{
		Method:  http.MethodPost,
		Path:    "/",
		Handler: postItem,
	}
	// DELETE /{id}
	deleteItemRoute := Route{
		Method:  http.MethodDelete,
		Path:    "/",
		Handler: deleteItem,
	}

	routes := []Route{getItemRoute, postItemRoute, deleteItemRoute}

	if request.Method != http.MethodGet && request.Method != http.MethodPost && request.Method != http.MethodDelete {
		http.Error(responseWriter, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	for _, route := range routes {
		if request.Method == route.Method && strings.HasPrefix(request.URL.Path, route.Path) {
			route.Handler(responseWriter, request)
			return
		}
	}

	http.Error(responseWriter, "Not found", http.StatusNotFound)
}

func main() {
	if !jsonDatabase.Exists() {
		err := jsonDatabase.CreateJsonFile()
		if err != nil {
			log.Println("json file create error", err)
			panic(err)
		}
	}

	http.HandleFunc("/", requestRouter)
	http.ListenAndServe(":8080", nil)
}
