package main

import (
	"fmt"
	"strings"
)

type Graph struct {
	vertices []*Vertex
}

type Vertex struct {
	key      string
	adjacent []*Vertex
}

func NewUndirectedGraph(vertices [][]string, graph *Graph) error {
	for _, v := range vertices {
		err := graph.AddVertex(v[0])
		if err != nil {
			return err
		}

		err = graph.AddVertex(v[1])
		if err != nil {
			return err
		}

		err = graph.AddEdge(v[0], v[1])
		if err != nil {
			return err
		}
	}

	return nil
}

func (g *Graph) AddVertex(key string) error {
	if contains(g.vertices, key) {
		return fmt.Errorf("Vertex %v already exists", key)
	}

	g.vertices = append(g.vertices, &Vertex{key: key})

	return nil
}

// Adds Edge in undirected Graph
func (g *Graph) AddEdge(key1, key2 string) error {
	vertex1 := g.getVertex(key1)
	vertex2 := g.getVertex(key2)
	if vertex1 == nil || vertex2 == nil {
		return fmt.Errorf("invalid edge (%v)<--->(%v)", key1, key2)
	}

	if contains(vertex1.adjacent, vertex2.key) || contains(vertex2.adjacent, vertex1.key) {
		return fmt.Errorf("edge already exists (%v)<--->(%v)", key1, key2)
	}

	vertex1.adjacent = append(vertex1.adjacent, vertex2)
	vertex2.adjacent = append(vertex2.adjacent, vertex1)

	return nil
}

func (g *Graph) Print() {
	for _, v := range g.vertices {
		fmt.Printf("\n%v : ", v.key)
		for _, a := range v.adjacent {
			fmt.Printf("%v ", a.key)
		}
	}

	fmt.Println()
}

func (g *Graph) getVertex(key string) *Vertex {
	for i, v := range g.vertices {
		if v.key == key {
			return g.vertices[i]
		}
	}

	return nil
}

func contains(vertices []*Vertex, k string) bool {
	for _, v := range vertices {
		if k == v.key {
			return true
		}
	}

	return false
}

// Function to find cycles of specific length in an undirected graph
func findCycles(graph *Graph, cycleLength int) map[string]string {
	cycles := make(map[string]string)

	var dfs func(vertex *Vertex, path []string, visited map[string]bool)
	dfs = func(vertex *Vertex, path []string, visited map[string]bool) {
		visited[vertex.key] = true
		path = append(path, vertex.key)

		for _, neighbor := range vertex.adjacent {
			if visited[neighbor.key] && neighbor.key != path[len(path)-2] {
				if len(path)-indexOf(path, neighbor.key) == cycleLength {
					cycle := strings.Join(path[indexOf(path, neighbor.key):], "-->")
					// cycle := append([]string{}, path[indexOf(path, neighbor.key):]...)
					cycles[cycle] = neighbor.key
				}
			} else if !visited[neighbor.key] {
				dfs(neighbor, path, visited)
			}
		}

		delete(visited, vertex.key)
		// path = path[:len(path)-1]
	}

	for _, startVertex := range graph.vertices {
		dfs(startVertex, []string{}, map[string]bool{})
	}

	return cycles
}

// Helper function to find the index of an element in a slice
func indexOf(slice []string, element string) int {
	for i, el := range slice {
		if el == element {
			return i
		}
	}
	return -1
}
