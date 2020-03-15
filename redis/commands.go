package redis

import (
	"github.com/gomodule/redigo/redis"
	"github.com/pkg/errors"
)

// Ping pings a Redis DB
func Ping() error {
	conn := Pool.Get()
	defer conn.Close()

	_, err := redis.String(conn.Do("PING"))
	if err != nil {
		return errors.Wrap(err, "cannot ping server")
	}
	return nil
}

// FlushAll deletes all the keys from a Redis DB
func FlushAll() error {
	conn := Pool.Get()
	defer conn.Close()

	_, err := redis.String(conn.Do("FLUSHALL"))
	if err != nil {
		return errors.Wrap(err, "cannot flush all")
	}
	return nil
}

// Set sets the value of a key in a Redis DB
func Set(key string, value interface{}) error {
	conn := Pool.Get()
	defer conn.Close()

	_, err := conn.Do("SET", key, value)
	if err != nil {
		var v interface{}

		switch value.(type) {
		case string:
			v := value.(string)
			if len(v) > 15 {
				v = v[0:12] + "..."
			}
			break
		default:
			v = value
		}

		return errors.Wrapf(err, "cannot set key %s to %v", key, v)
	}
	return nil
}

// GetString gets the string value of a key from a Redis DB
func GetString(key string) (value string, err error) {
	conn := Pool.Get()
	defer conn.Close()

	value, err = redis.String(conn.Do("GET", key))
	if err != nil {
		err = errors.Wrapf(err, "cannot get key %s", key)
	}
	return
}

// GetInt gets the int value of a key from a Redis DB
func GetInt(key string) (value int, err error) {
	conn := Pool.Get()
	defer conn.Close()

	value, err = redis.Int(conn.Do("GET", key))
	if err != nil {
		err = errors.Wrapf(err, "cannot get key %s", key)
	}
	return
}

// Exists check if a key exists in a Redis DB
func Exists(key string) (exists bool, err error) {
	conn := Pool.Get()
	defer conn.Close()

	exists, err = redis.Bool(conn.Do("EXISTS", key))
	if err != nil {
		err = errors.Wrapf(err, "cannot check if key %s exists", key)
	}
	return
}

// Delete deletes a key from a Redis DB
func Delete(key string) error {
	conn := Pool.Get()
	defer conn.Close()

	_, err := conn.Do("DEL", key)
	if err != nil {
		return errors.Wrapf(err, "cannot delete key %s", key)
	}
	return nil
}

// Expire sets the expiration of a key in a Redis DB
func Expire(key string, ttl int) error {
	conn := Pool.Get()
	defer conn.Close()

	_, err := conn.Do("EXPIRE", key, ttl)
	if err != nil {
		return errors.Wrapf(err, "cannot set key %s expiration to %d seconds", key, ttl)
	}
	return nil
}

// SetAddMember adds a member to a set in a Redis DB
func SetAddMember(key string, member interface{}) error {
	conn := Pool.Get()
	defer conn.Close()

	_, err := conn.Do("SADD", key, member)
	if err != nil {
		var m interface{}

		switch member.(type) {
		case string:
			m := member.(string)
			if len(m) > 15 {
				m = m[0:12] + "..."
			}
			break
		default:
			m = member
		}

		return errors.Wrapf(err, "Error adding member %v to set %s", m, key)
	}
	return nil
}

// GetSetMembers gets the members of a set from a Redis DB
func GetSetMembers(key string) (members []string, err error) {
	conn := Pool.Get()
	defer conn.Close()

	members, err = redis.Strings(conn.Do("SMEMBERS", key))
	if err != nil {
		err = errors.Wrapf(err, "cannot get members of set %s", key)
	}
	return
}

// IsSetMember returns whether "member" is a member of the set "key" or not
func IsSetMember(key string, member interface{}) (bool, error) {
	conn := Pool.Get()
	defer conn.Close()

	return redis.Bool(conn.Do("SISMEMBER", key, member))
}

// SetRemoveMember removes the member of a set from a Redis DB
func SetRemoveMember(key string, member interface{}) error {
	conn := Pool.Get()
	defer conn.Close()

	_, err := conn.Do("SREM", key, member)
	if err != nil {
		var m interface{}

		switch member.(type) {
		case string:
			m := member.(string)
			if len(m) > 15 {
				m = m[0:12] + "..."
			}
			break
		default:
			m = member
		}

		return errors.Wrapf(err, "Error removing member %v from set %s", m, key)
	}
	return nil
}
