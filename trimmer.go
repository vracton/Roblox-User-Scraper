package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func trim(file string) {
	data, err := os.ReadFile(file)
	if err != nil {
		fmt.Println("error reading file:", err)
		return
	}
	
	var userData []UserData
	err = json.Unmarshal(data, &userData)
	if err != nil {
		fmt.Println("error unmarshalling json:", err)
		return
	}

	numValid := 0

	for i := len(userData) - 1; i>=0; i-- {
		if !userData[i].Exists {
			userData = append(userData[:i], userData[i+1:]...)
		} else {
			numValid++
		}
	}

	trimmedData, err := json.Marshal(userData)
	if err != nil {
		fmt.Println("error marshalling json:", err)
		return
	}

	err = os.WriteFile("trimmed_"+file, trimmedData, 0644)
	if err != nil {
		fmt.Println("error writing file:", err)
		return
	}
	fmt.Printf("trimmed data to %d valid users\n", numValid)
}