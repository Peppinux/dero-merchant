package redis

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/peppinux/dero-merchant/config"
)

type CommandsTestSuite struct {
	suite.Suite
}

func (suite *CommandsTestSuite) SetupSuite() {
	err := config.LoadFromENV("../.env")
	if err != nil {
		panic(err)
	}

	Pool = NewPool(config.RedisAddress)
	err = Ping()
	if err != nil {
		panic(err)
	}

	err = FlushAll()
	if err != nil {
		panic(err)
	}
}

func (suite *CommandsTestSuite) TearDownSuite() {
	FlushAll()
	Pool.Close()
}

func TestCommandsTestSuite(t *testing.T) {
	suite.Run(t, new(CommandsTestSuite))
}

func (suite *CommandsTestSuite) TestPing() {
	err := Ping()
	suite.Nil(err)
}

func (suite *CommandsTestSuite) TestKeys() {
	stringKey := "test:string:foo"
	stringVal := "bar"

	intKey := "test:int:baz"
	intVal := 123

	unsetKey := "test:unset:key"

	// Set
	err := Set(stringKey, stringVal)
	suite.Nil(err)

	err = Set(intKey, intVal)
	suite.Nil(err)

	// Exists
	exists, _ := Exists(stringKey)
	suite.Nil(err)
	suite.True(exists)

	exists, _ = Exists(intKey)
	suite.Nil(err)
	suite.True(exists)

	exists, _ = Exists(unsetKey)
	suite.False(exists)

	// Get
	sVal, err := GetString(stringKey)
	suite.Nil(err)
	suite.Equal(stringVal, sVal)

	iVal, err := GetInt(intKey)
	suite.Nil(err)
	suite.Equal(intVal, iVal)

	_, err = GetString(unsetKey)
	suite.NotNil(err)

	// Flush all
	err = FlushAll()
	suite.Nil(err)

	// Exists after flushing all
	exists, _ = Exists(stringKey)
	suite.False(exists)

	exists, _ = Exists(intKey)
	suite.False(exists)

	// Get
	_, err = GetString(stringKey)
	suite.NotNil(err)

	_, err = GetInt(stringKey)
	suite.NotNil(err)

	// Set again in order to test Delete and Expire
	Set(stringKey, stringVal)
	Set(intKey, intVal)

	// Delete
	err = Delete(stringKey)
	suite.Nil(err)
	exists, _ = Exists(stringKey)
	suite.False(exists)
	sVal, err = GetString(stringKey)
	suite.NotNil(err)
	suite.Zero(sVal)

	// Expire
	err = Expire(intKey, 2)
	suite.Nil(err)
	exists, _ = Exists(intKey)
	suite.True(exists)
	iVal, err = GetInt(intKey)
	suite.Nil(err)
	suite.Equal(intVal, iVal)
	time.AfterFunc(time.Second*2, func() {
		exists, _ = Exists(intKey)
		suite.False(exists)
		iVal, err = GetInt(intKey)
		suite.NotNil(err)
		suite.Zero(iVal)
	})
}

func (suite *CommandsTestSuite) TestSets() {
	setKey := "test:set:foobar"
	setMembers := map[string]bool{
		"asd": true,
		"esd": true,
		"isd": true,
		"osd": false,
		"usd": false,
	}

	for k, v := range setMembers {
		// Add
		if v == true {
			err := SetAddMember(setKey, k)
			suite.Nil(err)
		}

		// Is member
		isMember, err := IsSetMember(setKey, k)
		suite.Nil(err)
		suite.Equal(v, isMember)
	}

	// Get
	members, err := GetSetMembers(setKey)
	suite.Nil(err)
	for _, m := range members {
		_, hasKey := setMembers[m]
		suite.True(hasKey)
	}

	// Remove
	removedMember := "asd"
	err = SetRemoveMember(setKey, removedMember)
	suite.Nil(err)
	members, _ = GetSetMembers(setKey)
	suite.NotContains(members, removedMember)

	// Delete
	err = Delete(setKey)
	suite.Nil(err)
	members, _ = GetSetMembers(setKey)
	suite.Equal([]string{}, members)
}
