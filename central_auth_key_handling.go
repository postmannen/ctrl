package ctrl

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"sync"

	"github.com/fxamacker/cbor/v2"
	bolt "go.etcd.io/bbolt"
)

// centralAuth holds the logic related to handling public keys and auth maps.
type centralAuth struct {
	// acl and authorization level related data and methods.
	accessLists *accessLists
	// public key distribution related data and methods.
	pki           *pki
	configuration *Configuration
	errorKernel   *errorKernel
}

// newCentralAuth will return a new and prepared *centralAuth
func newCentralAuth(configuration *Configuration, errorKernel *errorKernel) *centralAuth {
	c := centralAuth{
		configuration: configuration,
		errorKernel:   errorKernel,
	}
	c.pki = newPKI(configuration, errorKernel)
	c.accessLists = newAccessLists(c.pki, errorKernel, configuration)

	c.generateACLsForAllNodes()

	return &c
}

// nodesAcked is the structure that holds all the keys that we have
// acknowledged, and that are allowed to be distributed within the
// system. It also contains a hash of all those keys.
type nodesAcked struct {
	mu          sync.Mutex
	keysAndHash *keysAndHash
}

// newNodesAcked will return a prepared *nodesAcked structure.
func newNodesAcked() *nodesAcked {
	n := nodesAcked{
		keysAndHash: newKeysAndHash(),
	}

	return &n
}

// pki holds the data and method relevant to key handling and distribution.
type pki struct {
	nodesAcked             *nodesAcked
	nodeNotAckedPublicKeys *nodeNotAckedPublicKeys
	configuration          *Configuration
	db                     *bolt.DB
	bucketNamePublicKeys   string
	errorKernel            *errorKernel
}

// newKeys will return a prepared *keys with input values set.
func newPKI(configuration *Configuration, errorKernel *errorKernel) *pki {
	p := pki{
		// schema:           make(map[Node]map[argsString]signatureBase32),
		nodesAcked:             newNodesAcked(),
		nodeNotAckedPublicKeys: newNodeNotAckedPublicKeys(),
		configuration:          configuration,
		bucketNamePublicKeys:   "publicKeys",
		errorKernel:            errorKernel,
	}

	databaseFilepath := filepath.Join(configuration.DatabaseFolder, "auth.db")

	// Open the database file for persistent storage of public keys.
	db, err := bolt.Open(databaseFilepath, 0660, nil)
	if err != nil {
		errorKernel.logDebug("newPKI: error: failed to open db", "error", err)
		return &p
	}

	p.db = db

	// Get public keys from db storage.
	keys, err := p.dbDumpPublicKey()
	if err != nil {
		errorKernel.logDebug("newPKI: dbPublicKeyDump failed, probably empty db", "error", err)
	}

	// Only assign from storage to in memory map if the storage contained any values.
	if keys != nil {
		p.nodesAcked.keysAndHash.Keys = keys
		for k, v := range keys {
			errorKernel.logDebug("newPKI: public keys db contains", "key", k, "value", []byte(v))
		}
	}

	// Get the current hash from db if one exists.
	hash, err := p.dbViewHash()
	if err != nil {
		log.Printf("debug: dbViewHash failed: %v\n", err)
	}

	if hash != nil {
		var h [32]byte
		copy(h[:], hash)
		p.nodesAcked.keysAndHash.Hash = h
	}

	return &p
}

// addPublicKey to the db if the node do not exist, or if it is a new value.
func (c *centralAuth) addPublicKey(proc process, msg Message) {

	// Check if a key for the current node already exists in the map.
	c.pki.nodesAcked.mu.Lock()
	existingKey, ok := c.pki.nodesAcked.keysAndHash.Keys[msg.FromNode]
	c.pki.nodesAcked.mu.Unlock()

	if ok && bytes.Equal(existingKey, msg.Data) {
		proc.errorKernel.logDebug("addPublicKey: public key value for registered node is the same, doing nothing", "node", msg.FromNode)
		return
	}

	notAckedNodes := []Node{}
	c.pki.nodeNotAckedPublicKeys.mu.Lock()

	c.pki.nodeNotAckedPublicKeys.KeyMap[msg.FromNode] = msg.Data

	for k := range c.pki.nodeNotAckedPublicKeys.KeyMap {
		notAckedNodes = append(notAckedNodes, k)
	}

	c.pki.nodeNotAckedPublicKeys.mu.Unlock()

	er := fmt.Errorf("addPublicKey: key(s) needs to be allowed by operator for nodes: %v", notAckedNodes)
	c.pki.errorKernel.infoSend(proc, msg, er)
}

// deletePublicKeys to the db if the node do not exist, or if it is a new value.
func (c *centralAuth) deletePublicKeys(proc process, msg Message, nodes []string) {

	// Check if a key for the current node already exists in the map.
	func() {
		c.pki.nodesAcked.mu.Lock()
		defer c.pki.nodesAcked.mu.Unlock()

		for _, n := range nodes {
			delete(c.pki.nodesAcked.keysAndHash.Keys, Node(n))
		}
	}()

	err := c.pki.dbDeletePublicKeys(c.pki.bucketNamePublicKeys, nodes)
	if err != nil {
		proc.errorKernel.errSend(proc, msg, err, logWarning)
	}

	er := fmt.Errorf("info: detected new public key for node: %v. This key will need to be authorized by operator to be allowed into the system", msg.FromNode)
	c.pki.errorKernel.infoSend(proc, msg, er)
}

// dbUpdatePublicKey will update the public key for a node in the db.
func (p *pki) dbUpdatePublicKey(node string, value []byte) error {
	err := p.db.Update(func(tx *bolt.Tx) error {
		//Create a bucket
		bu, err := tx.CreateBucketIfNotExists([]byte(p.bucketNamePublicKeys))
		if err != nil {
			return fmt.Errorf("error: CreateBuckerIfNotExists failed: %v", err)
		}

		//Put a value into the bucket.
		if err := bu.Put([]byte(node), []byte(value)); err != nil {
			return err
		}

		//If all was ok, we should return a nil for a commit to happen. Any error
		// returned will do a rollback.
		return nil
	})
	return err
}

// dbDeletePublicKeys will delete the specified key from the specified
// bucket if it exists.
func (p *pki) dbDeletePublicKeys(bucket string, nodes []string) error {
	err := p.db.Update(func(tx *bolt.Tx) error {
		bu := tx.Bucket([]byte(bucket))

		for _, n := range nodes {
			err := bu.Delete([]byte(n))
			if err != nil {
				er := fmt.Errorf("error: delete key in bucket %v failed: %v", bucket, err)
				return er
			}
		}

		return nil
	})

	return err
}

// dbUpdateHash will update the public key for a node in the db.
func (p *pki) dbUpdateHash(hash []byte) error {
	err := p.db.Update(func(tx *bolt.Tx) error {
		//Create a bucket
		bu, err := tx.CreateBucketIfNotExists([]byte("hash"))
		if err != nil {
			return fmt.Errorf("error: CreateBuckerIfNotExists failed: %v", err)
		}

		//Put a value into the bucket.
		if err := bu.Put([]byte("hash"), []byte(hash)); err != nil {
			return err
		}

		//If all was ok, we should return a nil for a commit to happen. Any error
		// returned will do a rollback.
		return nil
	})
	return err
}

func (c *centralAuth) updateHash(proc process, message Message) {
	c.pki.nodesAcked.mu.Lock()
	defer c.pki.nodesAcked.mu.Unlock()

	type NodesAndKeys struct {
		Node Node
		Key  []byte
	}

	// Create a slice of all the map keys, and its value.
	sortedNodesAndKeys := []NodesAndKeys{}

	// Range the map, and add each k/v to the sorted slice, to be sorted later.
	for k, v := range c.pki.nodesAcked.keysAndHash.Keys {
		nk := NodesAndKeys{
			Node: k,
			Key:  v,
		}

		sortedNodesAndKeys = append(sortedNodesAndKeys, nk)
	}

	// sort the slice based on the node name.
	// Sort all the commands.
	sort.SliceStable(sortedNodesAndKeys, func(i, j int) bool {
		return sortedNodesAndKeys[i].Node < sortedNodesAndKeys[j].Node
	})

	// Then create a hash based on the sorted slice.

	b, err := cbor.Marshal(sortedNodesAndKeys)
	if err != nil {
		er := fmt.Errorf("error: updateHash, failed to marshal slice, and will not update hash for public keys:  %v", err)
		c.pki.errorKernel.errSend(proc, message, er, logError)

		return
	}

	// Store the key in the key value map.
	hash := sha256.Sum256(b)
	c.pki.nodesAcked.keysAndHash.Hash = hash

	// Store the key to the db for persistence.
	err = c.pki.dbUpdateHash(hash[:])
	if err != nil {
		er := fmt.Errorf("error: methodKeysAllow, failed to store the hash into the db:  %v", err)
		c.pki.errorKernel.errSend(proc, message, er, logError)

		return
	}

}

// dbViewHash will look up and return a specific value if it exists for a key in a bucket in a DB.
func (p *pki) dbViewHash() ([]byte, error) {
	var value []byte
	// View is a help function to get values out of the database.
	err := p.db.View(func(tx *bolt.Tx) error {
		//Open a bucket to get key's and values from.
		bu := tx.Bucket([]byte("hash"))
		if bu == nil {
			p.errorKernel.logWarn("no db hash bucket exist", "bucket", "hash")
			return nil
		}

		v := bu.Get([]byte("hash"))
		if len(v) == 0 {
			p.errorKernel.logWarn("dbViewHash: get bucket equals", "length", 0)
			return nil
		}

		value = v

		return nil
	})

	return value, err

}

// // deleteKeyFromBucket will delete the specified key from the specified
// // bucket if it exists.
// func (c *centralAuth) dbDeletePublicKey(key string) error {
// 	err := c.db.Update(func(tx *bolt.Tx) error {
// 		bu := tx.Bucket([]byte(c.bucketNamePublicKeys))
//
// 		err := bu.Delete([]byte(key))
// 		if err != nil {
// 			log.Printf("error: delete key in bucket %v failed: %v\n", c.bucketNamePublicKeys, err)
// 		}
//
// 		return nil
// 	})
//
// 	return err
// }

// dumpBucket will dump out all they keys and values in the
// specified bucket, and return a sorted []samDBValue
func (p *pki) dbDumpPublicKey() (map[Node][]byte, error) {
	m := make(map[Node][]byte)

	err := p.db.View(func(tx *bolt.Tx) error {
		bu := tx.Bucket([]byte(p.bucketNamePublicKeys))
		if bu == nil {
			return fmt.Errorf("error: dumpBucket: tx.bucket returned nil")
		}

		// For each element found in the DB, print it.
		bu.ForEach(func(k, v []byte) error {
			m[Node(k)] = v
			return nil
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return m, nil
}

// --- HERE

// nodeNotAckedPublicKeys holds all the gathered but not acknowledged public
// keys of nodes in the system.
type nodeNotAckedPublicKeys struct {
	mu     sync.RWMutex
	KeyMap map[Node][]byte
}

// newNodeNotAckedPublicKeys will return a prepared type of nodePublicKeys.
func newNodeNotAckedPublicKeys() *nodeNotAckedPublicKeys {
	n := nodeNotAckedPublicKeys{
		KeyMap: make(map[Node][]byte),
	}

	return &n
}
