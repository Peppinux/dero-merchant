package redis

import (
	"strconv"

	"github.com/pkg/errors"

	"github.com/peppinux/dero-merchant/stringutil"
)

// SetSessionUser sets userID as the value of sessionid:<sessionID>:userid key
func SetSessionUser(sessionID string, userID int) error {
	key := stringutil.Build("sessionid:", sessionID, ":userid")
	err := Set(key, userID)
	if err != nil {
		return errors.Wrap(err, "cannot set key in Redis")
	}
	return nil
}

// SetSessionExpiration sets key sessionid:<sessionID>:userid to expire after ttl seconds passed
func SetSessionExpiration(sessionID string, ttl int) error {
	key := stringutil.Build("sessionid:", sessionID, ":userid")
	err := Expire(key, ttl)
	if err != nil {
		return errors.Wrap(err, "cannot expire key in Redis")
	}
	return nil
}

// GetSessionUser returns the int value of key sessionid:<sessionID>:userid
func GetSessionUser(sessionID string) (userID int, err error) {
	key := stringutil.Build("sessionid:", sessionID, ":userid")
	userID, err = GetInt(key)
	if err != nil {
		err = errors.Wrap(err, "cannot get int value from Redis")
	}
	return
}

// DeleteSession deletes key sessionid:<sessionID>:userid
func DeleteSession(sessionID string) error {
	key := stringutil.Build("sessionid:", sessionID, ":userid")
	err := Delete(key)
	if err != nil {
		return errors.Wrap(err, "cannot delete key from Redis")
	}
	return nil
}

// AddUserSession adds userID to userid:<userID>:sessionids set
func AddUserSession(userID int, sessionID string) error {
	key := stringutil.Build("userid:", strconv.Itoa(userID), ":sessionids")
	err := SetAddMember(key, sessionID)
	if err != nil {
		return errors.Wrap(err, "cannot add member to set in Redis")
	}
	return nil
}

// RemoveUserSession removes sessionID from userid:<userID>:sessionids set
func RemoveUserSession(userID int, sessionID string) error {
	key := stringutil.Build("userid:", strconv.Itoa(userID), ":sessionids")
	err := SetRemoveMember(key, sessionID)
	if err != nil {
		return errors.Wrap(err, "cannot remove member from set in Redis")
	}
	return nil
}

// GetUserSessions returns all the values of set userid:<userID>:sessionids
func GetUserSessions(userID int) (sessionIDs []string, err error) {
	key := stringutil.Build("userid:", strconv.Itoa(userID), ":sessionids")
	sessionIDs, err = GetSetMembers(key)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get set members as strings from Redis")
	}
	return
}

// DeleteUserSessions removes set userid:<userID>:sessionids
func DeleteUserSessions(userID int) error {
	key := stringutil.Build("userid:", strconv.Itoa(userID), ":sessionids")
	err := Delete(key)
	if err != nil {
		return errors.Wrap(err, "cannot delete key in Redis")
	}
	return nil
}

// SetUserUsername sets username as the value of key userid:<userID>:username
func SetUserUsername(userID int, username string) error {
	key := stringutil.Build("userid:", strconv.Itoa(userID), ":username")
	err := Set(key, username)
	if err != nil {
		return errors.Wrap(err, "cannot set key in Redis")
	}
	return nil
}

// GetUserUsername returns the string value of key userid:<userID>:username
func GetUserUsername(userID int) (username string, err error) {
	key := stringutil.Build("userid:", strconv.Itoa(userID), ":username")
	username, err = GetString(key)
	if err != nil {
		err = errors.Wrap(err, "cannot get string value from Redis")
	}
	return
}

// SetUserEmail sets email as the value of key userid:<userID>:email
func SetUserEmail(userID int, email string) error {
	key := stringutil.Build("userid:", strconv.Itoa(userID), ":email")
	err := Set(key, email)
	if err != nil {
		return errors.Wrap(err, "cannot set key in Redis")
	}
	return nil
}

// GetUserEmail returns the string value of key userid:<userID>:email
func GetUserEmail(userID int) (email string, err error) {
	key := stringutil.Build("userid:", strconv.Itoa(userID), ":email")
	email, err = GetString(key)
	if err != nil {
		err = errors.Wrap(err, "cannot get string value from Redis")
	}
	return
}

// AddUserStore adds storeID to userid:<userID>:storeids set
func AddUserStore(userID, storeID int) error {
	key := stringutil.Build("userid:", strconv.Itoa(userID), ":storeids")
	err := SetAddMember(key, storeID)
	if err != nil {
		return errors.Wrap(err, "cannot add member to set in Redis")
	}
	return nil
}

// GetUserStores returns all the values of set userid:<userID>:storeids
func GetUserStores(userID int) (storeIDs []int, err error) {
	key := stringutil.Build("userid:", strconv.Itoa(userID), ":storeids")
	stores, err := GetSetMembers(key)
	if err != nil {
		err = errors.Wrap(err, "cannot get set members as strings from Redis")
	}

	var id int
	for _, s := range stores {
		id, err = strconv.Atoi(s)
		if err != nil {
			return nil, errors.Wrap(err, "cannot convert string to int")
		}

		storeIDs = append(storeIDs, id)
	}

	return
}

// UserOwnsStore returns whether storeID is a member of userid:<userID>:storeids set or not
func UserOwnsStore(userID, storeID int) (bool, error) {
	key := stringutil.Build("userid:", strconv.Itoa(userID), ":storeids")
	return IsSetMember(key, storeID)
}

// RemoveUserStore removes storeID from userid:<userID>:storeids set
func RemoveUserStore(userID, storeID int) error {
	key := stringutil.Build("userid:", strconv.Itoa(userID), ":storeids")
	err := SetRemoveMember(key, storeID)
	if err != nil {
		return errors.Wrap(err, "cannot remove member from set in Redis")
	}
	return nil
}

// SetStoreTitle sets title as the value of storeid:<storeID>:title key
func SetStoreTitle(storeID int, title string) error {
	key := stringutil.Build("storeid:", strconv.Itoa(storeID), ":title")
	err := Set(key, title)
	if err != nil {
		return errors.Wrap(err, "cannot set key value in Redis")
	}
	return nil
}

// GetStoreTitle returns the string value of key storeid:<storeID>:title
func GetStoreTitle(storeID int) (title string, err error) {
	key := stringutil.Build("storeid:", strconv.Itoa(storeID), ":title")
	title, err = GetString(key)
	if err != nil {
		err = errors.Wrap(err, "cannot get string value from Redis")
	}
	return
}

// DeleteStoreTitle deletes storeid:<storeid>:title key
func DeleteStoreTitle(storeID int) error {
	key := stringutil.Build("storeid:", strconv.Itoa(storeID), ":title")
	err := Delete(key)
	if err != nil {
		return errors.Wrap(err, "cannot delete key from Redis")
	}
	return nil
}

// SetAPIKeyStore sets storeID as the value of apikey:<apiKey>:storeid
func SetAPIKeyStore(apiKey string, storeID int) error {
	key := stringutil.Build("apikey:", apiKey, ":storeid")
	err := Set(key, storeID)
	if err != nil {
		return errors.Wrap(err, "cannot set key in Redis")
	}
	return nil
}

// GetAPIKeyStore returns the int value of apikey:<apiKey>:storeid
func GetAPIKeyStore(apiKey string) (storeID int, err error) {
	key := stringutil.Build("apikey:", apiKey, ":storeid")
	storeID, err = GetInt(key)
	if err != nil {
		err = errors.Wrap(err, "cannot get int value from Redis")
	}
	return
}

// DeleteAPIKeyStore deletes key apikey:<apiKey>:storeid
func DeleteAPIKeyStore(apiKey string) error {
	key := stringutil.Build("apikey:", apiKey, ":storeid")
	err := Delete(key)
	if err != nil {
		return errors.Wrap(err, "cannot delete key from Redis")
	}
	return nil
}

// SetAPIKeySecretKey sets secretKey as the value of apikey:<apiKey>:secretkey
func SetAPIKeySecretKey(apiKey, secretKey string) error {
	key := stringutil.Build("apikey:", apiKey, ":secretkey")
	err := Set(key, secretKey)
	if err != nil {
		return errors.Wrap(err, "cannot set key in Redis")
	}
	return nil
}

// GetAPIKeySecretKey returns the string value of apikey:<apiKey>:secretkey
func GetAPIKeySecretKey(apiKey string) (secretKey string, err error) {
	key := stringutil.Build("apikey:", apiKey, ":secretkey")
	secretKey, err = GetString(key)
	if err != nil {
		err = errors.Wrap(err, "cannot get string value from Redis")
	}
	return
}

// DeleteAPIKeySecretKey deletes key apikey:<apiKey>:secretkey
func DeleteAPIKeySecretKey(apiKey string) error {
	key := stringutil.Build("apikey:", apiKey, ":secretkey")
	err := Delete(key)
	if err != nil {
		return errors.Wrap(err, "cannot delete key from Redis")
	}
	return nil
}

// SetSupportedCurrencies adds all currencies to the supportedcurrencies set
func SetSupportedCurrencies(currencies []string) error {
	for _, c := range currencies {
		err := SetAddMember("supportedcurrencies", c)
		if err != nil {
			return errors.Wrap(err, "cannot add member to set in Redis")
		}
	}
	return nil
}

// IsSupportedCurrency returns whether currency is a member of the supportedcurrencies set or not
func IsSupportedCurrency(currency string) (bool, error) {
	return IsSetMember("supportedcurrencies", currency)
}
