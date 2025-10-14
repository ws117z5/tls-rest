package auth

import (
	"fmt"
	//"log"
	"reflect"
	"strconv"

	"github.com/ws117z5/tls-rest/go/constants"
	"github.com/ws117z5/tls-rest/go/lib/db/pgdb"
)

func GetRights(groupId, userId int) (map[int]int, error) {
	// This function should return a map of user rights based on groupId and userId
	// For now, we will return an empty map and nil error
	rights := make(map[int]int)

	db, err := pgdb.GetInstance()

	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	groupRights, _ := db.GetAll("SELECT module_id, rights FROM user_group_rights WHERE group_id = ?", groupId)

	userRights, _ := db.GetAll("SELECT module_id, rights FROM user_rights WHERE user_id = ?", userId)

	for _, groupRight := range groupRights {
		moduleId := groupRight["module_id"].(int)
		right := groupRight["rights"].(int)
		rights[moduleId] = right
	}
	for _, userRight := range userRights {
		moduleId := userRight["module_id"].(int)
		right := userRight["rights"].(int)
		if existingRight, exists := rights[moduleId]; exists {
			// Combine rights if they already exist
			rights[moduleId] = existingRight | right
		} else {
			// Add new right
			rights[moduleId] = right
		}
	}

	return rights, nil
}

func reflectStructField(Iface interface{}, FieldName string) error {
	ValueIface := reflect.ValueOf(Iface)

	// Check if the passed interface is a pointer
	if ValueIface.Type().Kind() != reflect.Ptr {
		// Create a new type of Iface's Type, so we have a pointer to work with
		ValueIface = reflect.New(reflect.TypeOf(Iface))
	}

	// 'dereference' with Elem() and get the field by name
	Field := ValueIface.Elem().FieldByName(FieldName)
	if !Field.IsValid() {
		return fmt.Errorf("Interface `%s` does not have the field `%s`", ValueIface.Type(), FieldName)
	}
	return nil
}

func convertRights(rights string) int {
	rightsValue := 0
	if rightsValue, err := strconv.ParseInt(rights, 2, 64); err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(rightsValue)
	}

	return rightsValue
}

// HasRight checks if user has access to this function
func HasRight(name string, rights string) bool {
	currentRightsValue := convertRights(rights)

	if module, err := constants.Config.GetModule(name); err != nil {
		convertRights(module.RightsMask)
		//err := reflectStructField(constants.Config["modules"].(map[string]interface{}), "righsMask")
		//moduleRightsValue := convertRights(moduleRights.rightsMask)
		return currentRightsValue&convertRights(module.RightsMask) == currentRightsValue
	}

	return false
}
