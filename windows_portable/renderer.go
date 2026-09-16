package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"math/rand"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Vec3 struct{ X, Y, Z float64 }

func V(x, y, z float64) Vec3      { return Vec3{x, y, z} }
func (a Vec3) Add(b Vec3) Vec3    { return V(a.X+b.X, a.Y+b.Y, a.Z+b.Z) }
func (a Vec3) Sub(b Vec3) Vec3    { return V(a.X-b.X, a.Y-b.Y, a.Z-b.Z) }
func (a Vec3) Mul(s float64) Vec3 { return V(a.X*s, a.Y*s, a.Z*s) }
func (a Vec3) Had(b Vec3) Vec3    { return V(a.X*b.X, a.Y*b.Y, a.Z*b.Z) }
func (a Vec3) Dot(b Vec3) float64 { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }
func (a Vec3) Cross(b Vec3) Vec3  { return V(a.Y*b.Z-a.Z*b.Y, a.Z*b.X-a.X*b.Z, a.X*b.Y-a.Y*b.X) }
func (a Vec3) Len2() float64      { return a.Dot(a) }
func (a Vec3) Len() float64       { return math.Sqrt(a.Len2()) }
func (a Vec3) Unit() Vec3 {
	l := a.Len()
	if l < 1e-12 {
		return V(0, 0, 0)
	}
	return a.Mul(1 / l)
}
func (a Vec3) MaxComp() float64 { return math.Max(a.X, math.Max(a.Y, a.Z)) }
func clamp(x, a, b float64) float64 {
	if x < a {
		return a
	}
	if x > b {
		return b
	}
	return x
}

type Ray struct{ O, D Vec3 }

type RayAccel struct {
	Ray                 Ray
	Inv                 Vec3
	DirLen2, InvDirLen2 float64
	ZeroX, ZeroY, ZeroZ bool
}

func prepareRay(r Ray) RayAccel {
	a := RayAccel{Ray: r}
	a.DirLen2 = r.D.Len2()
	if a.DirLen2 > 1e-18 {
		a.InvDirLen2 = 1 / a.DirLen2
	}
	if math.Abs(r.D.X) < 1e-15 {
		a.ZeroX = true
	} else {
		a.Inv.X = 1 / r.D.X
	}
	if math.Abs(r.D.Y) < 1e-15 {
		a.ZeroY = true
	} else {
		a.Inv.Y = 1 / r.D.Y
	}
	if math.Abs(r.D.Z) < 1e-15 {
		a.ZeroZ = true
	} else {
		a.Inv.Z = 1 / r.D.Z
	}
	return a
}

func (r Ray) At(t float64) Vec3 { return r.O.Add(r.D.Mul(t)) }

type MaterialKind int

const (
	MatDiffuse MaterialKind = iota
	MatMetal
	MatGlass
	MatEmissive
)

type Material struct {
	Kind      MaterialKind
	Color     Vec3
	Roughness float64
	IOR       float64
	Emission  Vec3
}

type PrimKind int

const (
	PrimSphere PrimKind = iota
	PrimTriangle
)

type Primitive struct {
	Kind           PrimKind
	Material       int
	Center         Vec3
	Radius         float64
	A, B, C        Vec3
	E1, E2, Normal Vec3
	Box            AABB
	Centroid       Vec3
}

type AABB struct{ Min, Max Vec3 }

func emptyAABB() AABB { inf := math.Inf(1); return AABB{V(inf, inf, inf), V(-inf, -inf, -inf)} }
func (b AABB) ExpandPoint(p Vec3) AABB {
	b.Min.X = math.Min(b.Min.X, p.X)
	b.Min.Y = math.Min(b.Min.Y, p.Y)
	b.Min.Z = math.Min(b.Min.Z, p.Z)
	b.Max.X = math.Max(b.Max.X, p.X)
	b.Max.Y = math.Max(b.Max.Y, p.Y)
	b.Max.Z = math.Max(b.Max.Z, p.Z)
	return b
}
func (b AABB) ExpandBox(c AABB) AABB { b = b.ExpandPoint(c.Min); b = b.ExpandPoint(c.Max); return b }
func (b AABB) SurfaceArea() float64 {
	e := b.Max.Sub(b.Min)
	if e.X < 0 || e.Y < 0 || e.Z < 0 {
		return 0
	}
	return 2 * (e.X*e.Y + e.Y*e.Z + e.Z*e.X)
}

func (b AABB) Hit(r Ray, tmin, tmax float64) bool {
	ar := prepareRay(r)
	ok, _ := b.HitAccel(&ar, tmin, tmax)
	return ok
}

func (b *AABB) HitAccel(r *RayAccel, tmin, tmax float64) (bool, float64) {
	near := tmin
	if r.ZeroX {
		if r.Ray.O.X < b.Min.X || r.Ray.O.X > b.Max.X {
			return false, 0
		}
	} else {
		t0 := (b.Min.X - r.Ray.O.X) * r.Inv.X
		t1 := (b.Max.X - r.Ray.O.X) * r.Inv.X
		if r.Inv.X < 0 {
			t0, t1 = t1, t0
		}
		if t0 > near {
			near = t0
		}
		if t1 < tmax {
			tmax = t1
		}
		if tmax <= near {
			return false, 0
		}
	}
	if r.ZeroY {
		if r.Ray.O.Y < b.Min.Y || r.Ray.O.Y > b.Max.Y {
			return false, 0
		}
	} else {
		t0 := (b.Min.Y - r.Ray.O.Y) * r.Inv.Y
		t1 := (b.Max.Y - r.Ray.O.Y) * r.Inv.Y
		if r.Inv.Y < 0 {
			t0, t1 = t1, t0
		}
		if t0 > near {
			near = t0
		}
		if t1 < tmax {
			tmax = t1
		}
		if tmax <= near {
			return false, 0
		}
	}
	if r.ZeroZ {
		if r.Ray.O.Z < b.Min.Z || r.Ray.O.Z > b.Max.Z {
			return false, 0
		}
	} else {
		t0 := (b.Min.Z - r.Ray.O.Z) * r.Inv.Z
		t1 := (b.Max.Z - r.Ray.O.Z) * r.Inv.Z
		if r.Inv.Z < 0 {
			t0, t1 = t1, t0
		}
		if t0 > near {
			near = t0
		}
		if t1 < tmax {
			tmax = t1
		}
		if tmax <= near {
			return false, 0
		}
	}
	return true, near
}

type BVHNode struct {
	Box          AABB
	Left, Right  int
	Start, Count int
}

type SceneObject struct {
	Name      string
	PrimStart int
	PrimCount int
	Pivot     Vec3
}

type Scene struct {
	Materials           []Material
	Primitives          []Primitive
	Nodes               []BVHNode
	Order               []int
	LightSpheres        []int
	LightCDF            []float64
	LightPDFByPrim      []float64
	Objects             []SceneObject
	Name                string
	BVHMode             string
	BVHBins             int
	BVHLeafSize         int
	GPUTopologyRevision uint64
	GPUGeometryRevision uint64
}

func (s *Scene) AddMaterial(m Material) int {
	s.Materials = append(s.Materials, m)
	return len(s.Materials) - 1
}
func (s *Scene) AddSphere(c Vec3, r float64, mat int) {
	if r <= 0 {
		return
	}
	rv := V(r, r, r)
	p := Primitive{Kind: PrimSphere, Material: mat, Center: c, Radius: r, Centroid: c, Box: AABB{c.Sub(rv), c.Add(rv)}}
	s.Primitives = append(s.Primitives, p)
	if mat >= 0 && mat < len(s.Materials) && s.Materials[mat].Kind == MatEmissive {
		s.LightSpheres = append(s.LightSpheres, len(s.Primitives)-1)
	}
}
func (s *Scene) AddTriangle(a, b, c Vec3, mat int) {
	e1 := b.Sub(a)
	e2 := c.Sub(a)
	n := e1.Cross(e2)
	if n.Len2() < 1e-18 {
		return
	}
	box := emptyAABB().ExpandPoint(a).ExpandPoint(b).ExpandPoint(c)
	eps := 1e-6
	box.Min = box.Min.Sub(V(eps, eps, eps))
	box.Max = box.Max.Add(V(eps, eps, eps))
	s.Primitives = append(s.Primitives, Primitive{Kind: PrimTriangle, Material: mat, A: a, B: b, C: c, E1: e1, E2: e2, Normal: n.Unit(), Box: box, Centroid: a.Add(b).Add(c).Mul(1.0 / 3.0)})
}

func (s *Scene) AddPresetObject(kind string, pos Vec3) int {
	start := len(s.Primitives)
	name := kind
	switch strings.ToLower(kind) {
	case "diffuse sphere":
		m := s.AddMaterial(Material{Kind: MatDiffuse, Color: V(0.72, 0.24, 0.18)})
		s.AddSphere(pos, 0.8, m)
	case "metal sphere":
		m := s.AddMaterial(Material{Kind: MatMetal, Color: V(0.78, 0.82, 0.9), Roughness: 0.12})
		s.AddSphere(pos, 0.8, m)
	case "glass sphere":
		m := s.AddMaterial(Material{Kind: MatGlass, Color: V(0.98, 0.99, 1.0), IOR: 1.5})
		s.AddSphere(pos, 0.8, m)
	case "light sphere":
		m := s.AddMaterial(Material{Kind: MatEmissive, Emission: V(18, 15, 11)})
		s.AddSphere(pos, 0.55, m)
	case "cube":
		m := s.AddMaterial(Material{Kind: MatDiffuse, Color: V(0.28, 0.52, 0.82)})
		h := 0.75
		v := []Vec3{
			pos.Add(V(-h, -h, -h)), pos.Add(V(h, -h, -h)), pos.Add(V(h, h, -h)), pos.Add(V(-h, h, -h)),
			pos.Add(V(-h, -h, h)), pos.Add(V(h, -h, h)), pos.Add(V(h, h, h)), pos.Add(V(-h, h, h)),
		}
		faces := [][3]int{{0, 1, 2}, {0, 2, 3}, {4, 6, 5}, {4, 7, 6}, {0, 4, 5}, {0, 5, 1}, {3, 2, 6}, {3, 6, 7}, {0, 3, 7}, {0, 7, 4}, {1, 5, 6}, {1, 6, 2}}
		for _, f := range faces {
			s.AddTriangle(v[f[0]], v[f[1]], v[f[2]], m)
		}
	default:
		return -1
	}
	count := len(s.Primitives) - start
	if count <= 0 {
		return -1
	}
	s.Objects = append(s.Objects, SceneObject{Name: name, PrimStart: start, PrimCount: count, Pivot: pos})
	s.Build()
	return len(s.Objects) - 1
}

func translatePrimitive(p *Primitive, d Vec3) {
	if p.Kind == PrimSphere {
		p.Center = p.Center.Add(d)
		p.Centroid = p.Center
		rv := V(p.Radius, p.Radius, p.Radius)
		p.Box = AABB{p.Center.Sub(rv), p.Center.Add(rv)}
		return
	}
	p.A = p.A.Add(d)
	p.B = p.B.Add(d)
	p.C = p.C.Add(d)
	p.E1 = p.B.Sub(p.A)
	p.E2 = p.C.Sub(p.A)
	p.Normal = p.E1.Cross(p.E2).Unit()
	p.Centroid = p.A.Add(p.B).Add(p.C).Mul(1.0 / 3.0)
	box := emptyAABB().ExpandPoint(p.A).ExpandPoint(p.B).ExpandPoint(p.C)
	eps := 1e-6
	box.Min = box.Min.Sub(V(eps, eps, eps))
	box.Max = box.Max.Add(V(eps, eps, eps))
	p.Box = box
}

func (s *Scene) TranslateObject(index int, d Vec3) bool {
	if index < 0 || index >= len(s.Objects) {
		return false
	}
	o := &s.Objects[index]
	end := minInt(len(s.Primitives), o.PrimStart+o.PrimCount)
	for i := o.PrimStart; i < end; i++ {
		translatePrimitive(&s.Primitives[i], d)
	}
	o.Pivot = o.Pivot.Add(d)
	return true
}

func (s *Scene) PickObject(ray Ray) (int, Hit, bool) {
	closest := 1e30
	bestObj := -1
	var best Hit
	ar := prepareRay(ray)
	for oi, o := range s.Objects {
		end := minInt(len(s.Primitives), o.PrimStart+o.PrimCount)
		for pi := o.PrimStart; pi < end; pi++ {
			if h, ok := primHitAccel(&s.Primitives[pi], &ar, 1e-4, closest); ok {
				closest = h.T
				best = h
				bestObj = oi
			}
		}
	}
	return bestObj, best, bestObj >= 0
}

func (s *Scene) rebuildLightDistribution() {
	if cap(s.LightCDF) < len(s.LightSpheres) {
		s.LightCDF = make([]float64, len(s.LightSpheres))
	} else {
		s.LightCDF = s.LightCDF[:len(s.LightSpheres)]
	}
	if cap(s.LightPDFByPrim) < len(s.Primitives) {
		s.LightPDFByPrim = make([]float64, len(s.Primitives))
	} else {
		s.LightPDFByPrim = s.LightPDFByPrim[:len(s.Primitives)]
		clear(s.LightPDFByPrim)
	}
	total := 0.0
	for i, pi := range s.LightSpheres {
		w := lightWeight(s, pi)
		total += w
		s.LightCDF[i] = total
	}
	if total <= 0 {
		if len(s.LightSpheres) == 0 {
			return
		}
		pdf := 1 / float64(len(s.LightSpheres))
		for i, pi := range s.LightSpheres {
			s.LightCDF[i] = float64(i+1) / float64(len(s.LightSpheres))
			if pi >= 0 && pi < len(s.LightPDFByPrim) {
				s.LightPDFByPrim[pi] = pdf
			}
		}
		return
	}
	prev := 0.0
	for i, pi := range s.LightSpheres {
		cdf := s.LightCDF[i] / total
		s.LightCDF[i] = cdf
		pdf := cdf - prev
		prev = cdf
		if pi >= 0 && pi < len(s.LightPDFByPrim) {
			s.LightPDFByPrim[pi] = pdf
		}
	}
	if len(s.LightCDF) > 0 {
		s.LightCDF[len(s.LightCDF)-1] = 1
	}
}

func (s *Scene) Build() {
	s.GPUTopologyRevision++
	s.GPUGeometryRevision++
	s.rebuildLightDistribution()
	if s.BVHMode == "" {
		s.BVHMode = "SAH"
	}
	if s.BVHBins < 4 {
		s.BVHBins = 16
	}
	if s.BVHBins > 32 {
		s.BVHBins = 32
	}
	if s.BVHLeafSize < 1 {
		s.BVHLeafSize = 4
	}
	if s.BVHLeafSize > 16 {
		s.BVHLeafSize = 16
	}
	neededNodes := len(s.Primitives) * 2
	if cap(s.Nodes) < neededNodes {
		s.Nodes = make([]BVHNode, 0, neededNodes)
	} else {
		s.Nodes = s.Nodes[:0]
	}
	if cap(s.Order) < len(s.Primitives) {
		s.Order = make([]int, len(s.Primitives))
	} else {
		s.Order = s.Order[:len(s.Primitives)]
	}
	for i := range s.Order {
		s.Order[i] = i
	}
	if len(s.Order) > 0 {
		if strings.EqualFold(s.BVHMode, "median") {
			s.buildNodeMedian(0, len(s.Order))
		} else {
			s.buildNodeSAH(0, len(s.Order))
		}
	}
}

func (s *Scene) Refit() {
	s.GPUGeometryRevision++
	if len(s.Nodes) == 0 {
		s.Build()
		return
	}
	for ni := len(s.Nodes) - 1; ni >= 0; ni-- {
		n := &s.Nodes[ni]
		if n.Count > 0 {
			box := emptyAABB()
			for i := 0; i < n.Count; i++ {
				pi := s.Order[n.Start+i]
				if pi >= 0 && pi < len(s.Primitives) {
					box = box.ExpandBox(s.Primitives[pi].Box)
				}
			}
			n.Box = box
		} else if n.Left >= 0 && n.Right >= 0 {
			n.Box = s.Nodes[n.Left].Box.ExpandBox(s.Nodes[n.Right].Box)
		}
	}
	s.rebuildLightDistribution()
}

func centroidAxis(v Vec3, axis int) float64 {
	if axis == 0 {
		return v.X
	}
	if axis == 1 {
		return v.Y
	}
	return v.Z
}

func largestAxis(ext Vec3) int {
	if ext.Y > ext.X && ext.Y >= ext.Z {
		return 1
	}
	if ext.Z > ext.X && ext.Z > ext.Y {
		return 2
	}
	return 0
}

func (s *Scene) nodeBounds(start, end int) (AABB, AABB) {
	box := emptyAABB()
	centroids := emptyAABB()
	for i := start; i < end; i++ {
		p := &s.Primitives[s.Order[i]]
		box = box.ExpandBox(p.Box)
		centroids = centroids.ExpandPoint(p.Centroid)
	}
	return box, centroids
}

func (s *Scene) buildNodeMedian(start, end int) int {
	idx := len(s.Nodes)
	s.Nodes = append(s.Nodes, BVHNode{Left: -1, Right: -1})
	box, cb := s.nodeBounds(start, end)
	count := end - start
	if count <= s.BVHLeafSize {
		s.Nodes[idx] = BVHNode{Box: box, Left: -1, Right: -1, Start: start, Count: count}
		return idx
	}
	axis := largestAxis(cb.Max.Sub(cb.Min))
	sort.Slice(s.Order[start:end], func(i, j int) bool {
		return centroidAxis(s.Primitives[s.Order[start+i]].Centroid, axis) < centroidAxis(s.Primitives[s.Order[start+j]].Centroid, axis)
	})
	mid := (start + end) / 2
	left := s.buildNodeMedian(start, mid)
	right := s.buildNodeMedian(mid, end)
	s.Nodes[idx] = BVHNode{Box: box, Left: left, Right: right}
	return idx
}

func (s *Scene) buildNodeSAH(start, end int) int {
	idx := len(s.Nodes)
	s.Nodes = append(s.Nodes, BVHNode{Left: -1, Right: -1})
	box, cb := s.nodeBounds(start, end)
	count := end - start
	if count <= s.BVHLeafSize {
		s.Nodes[idx] = BVHNode{Box: box, Left: -1, Right: -1, Start: start, Count: count}
		return idx
	}
	ext := cb.Max.Sub(cb.Min)
	axis := largestAxis(ext)
	axisExtent := centroidAxis(cb.Max, axis) - centroidAxis(cb.Min, axis)
	if axisExtent < 1e-12 {
		s.Nodes[idx] = BVHNode{Box: box, Left: -1, Right: -1, Start: start, Count: count}
		return idx
	}
	binsN := s.BVHBins
	type binData struct {
		box   AABB
		count int
	}
	var bins [32]binData
	for i := 0; i < binsN; i++ {
		bins[i].box = emptyAABB()
	}
	minC := centroidAxis(cb.Min, axis)
	invExtent := 1 / axisExtent
	for i := start; i < end; i++ {
		p := &s.Primitives[s.Order[i]]
		b := int((centroidAxis(p.Centroid, axis) - minC) * invExtent * float64(binsN))
		if b < 0 {
			b = 0
		}
		if b >= binsN {
			b = binsN - 1
		}
		bins[b].count++
		bins[b].box = bins[b].box.ExpandBox(p.Box)
	}
	var leftCount [31]int
	var rightCount [31]int
	var leftArea [31]float64
	var rightArea [31]float64
	accBox := emptyAABB()
	accCount := 0
	for i := 0; i < binsN-1; i++ {
		if bins[i].count > 0 {
			accBox = accBox.ExpandBox(bins[i].box)
		}
		accCount += bins[i].count
		leftCount[i] = accCount
		leftArea[i] = accBox.SurfaceArea()
	}
	accBox = emptyAABB()
	accCount = 0
	for i := binsN - 1; i > 0; i-- {
		if bins[i].count > 0 {
			accBox = accBox.ExpandBox(bins[i].box)
		}
		accCount += bins[i].count
		rightCount[i-1] = accCount
		rightArea[i-1] = accBox.SurfaceArea()
	}
	bestSplit := -1
	bestCost := math.Inf(1)
	for i := 0; i < binsN-1; i++ {
		if leftCount[i] == 0 || rightCount[i] == 0 {
			continue
		}
		cost := leftArea[i]*float64(leftCount[i]) + rightArea[i]*float64(rightCount[i])
		if cost < bestCost {
			bestCost, bestSplit = cost, i
		}
	}
	leafCost := box.SurfaceArea() * float64(count)
	if bestSplit < 0 || (count <= s.BVHLeafSize*2 && bestCost >= leafCost*0.98) {
		s.Nodes[idx] = BVHNode{Box: box, Left: -1, Right: -1, Start: start, Count: count}
		return idx
	}
	i, j := start, end-1
	for i <= j {
		p := &s.Primitives[s.Order[i]]
		b := int((centroidAxis(p.Centroid, axis) - minC) * invExtent * float64(binsN))
		if b < 0 {
			b = 0
		}
		if b >= binsN {
			b = binsN - 1
		}
		if b <= bestSplit {
			i++
		} else {
			s.Order[i], s.Order[j] = s.Order[j], s.Order[i]
			j--
		}
	}
	mid := i
	if mid <= start || mid >= end {
		sort.Slice(s.Order[start:end], func(a, b int) bool {
			return centroidAxis(s.Primitives[s.Order[start+a]].Centroid, axis) < centroidAxis(s.Primitives[s.Order[start+b]].Centroid, axis)
		})
		mid = (start + end) / 2
	}
	left := s.buildNodeSAH(start, mid)
	right := s.buildNodeSAH(mid, end)
	s.Nodes[idx] = BVHNode{Box: box, Left: left, Right: right}
	return idx
}

type Hit struct {
	T         float64
	N         Vec3
	Material  int
	Primitive int
	Front     bool
}

type Guide struct {
	P, N   Vec3
	Albedo Vec3
	Depth  float64
	Valid  bool
}

func sphereHitAccel(p *Primitive, ar *RayAccel, tmin, tmax float64) (Hit, bool) {
	if ar.InvDirLen2 <= 0 {
		return Hit{}, false
	}
	r := ar.Ray
	oc := r.O.Sub(p.Center)
	hb := oc.Dot(r.D)
	c := oc.Len2() - p.Radius*p.Radius
	disc := hb*hb - ar.DirLen2*c
	if disc < 0 {
		return Hit{}, false
	}
	sq := math.Sqrt(disc)
	root := (-hb - sq) * ar.InvDirLen2
	if root < tmin || root > tmax {
		root = (-hb + sq) * ar.InvDirLen2
		if root < tmin || root > tmax {
			return Hit{}, false
		}
	}
	pos := r.At(root)
	outward := pos.Sub(p.Center).Mul(1 / p.Radius)
	front := r.D.Dot(outward) < 0
	n := outward
	if !front {
		n = n.Mul(-1)
	}
	return Hit{T: root, N: n, Material: p.Material, Primitive: -1, Front: front}, true
}

func sphereHit(p *Primitive, r Ray, tmin, tmax float64) (Hit, bool) {
	ar := prepareRay(r)
	return sphereHitAccel(p, &ar, tmin, tmax)
}
func triHit(p *Primitive, r Ray, tmin, tmax float64) (Hit, bool) {
	h := r.D.Cross(p.E2)
	det := p.E1.Dot(h)
	if math.Abs(det) < 1e-10 {
		return Hit{}, false
	}
	f := 1 / det
	ss := r.O.Sub(p.A)
	u := f * ss.Dot(h)
	if u < 0 || u > 1 {
		return Hit{}, false
	}
	q := ss.Cross(p.E1)
	v := f * r.D.Dot(q)
	if v < 0 || u+v > 1 {
		return Hit{}, false
	}
	t := f * p.E2.Dot(q)
	if t < tmin || t > tmax {
		return Hit{}, false
	}
	n := p.Normal
	front := r.D.Dot(n) < 0
	if !front {
		n = n.Mul(-1)
	}
	return Hit{T: t, N: n, Material: p.Material, Primitive: -1, Front: front}, true
}
func primHitAccel(p *Primitive, ar *RayAccel, tmin, tmax float64) (Hit, bool) {
	if p.Kind == PrimSphere {
		return sphereHitAccel(p, ar, tmin, tmax)
	}
	return triHit(p, ar.Ray, tmin, tmax)
}

func primHit(p *Primitive, r Ray, tmin, tmax float64) (Hit, bool) {
	ar := prepareRay(r)
	return primHitAccel(p, &ar, tmin, tmax)
}

func sphereAnyHitAccel(p *Primitive, ar *RayAccel, tmin, tmax float64) bool {
	if ar.InvDirLen2 <= 0 {
		return false
	}
	oc := ar.Ray.O.Sub(p.Center)
	hb := oc.Dot(ar.Ray.D)
	c := oc.Len2() - p.Radius*p.Radius
	disc := hb*hb - ar.DirLen2*c
	if disc < 0 {
		return false
	}
	sq := math.Sqrt(disc)
	root := (-hb - sq) * ar.InvDirLen2
	if root >= tmin && root <= tmax {
		return true
	}
	root = (-hb + sq) * ar.InvDirLen2
	return root >= tmin && root <= tmax
}

func triAnyHit(p *Primitive, r Ray, tmin, tmax float64) bool {
	h := r.D.Cross(p.E2)
	det := p.E1.Dot(h)
	if det > -1e-10 && det < 1e-10 {
		return false
	}
	f := 1 / det
	ss := r.O.Sub(p.A)
	u := f * ss.Dot(h)
	if u < 0 || u > 1 {
		return false
	}
	q := ss.Cross(p.E1)
	v := f * r.D.Dot(q)
	if v < 0 || u+v > 1 {
		return false
	}
	t := f * p.E2.Dot(q)
	return t >= tmin && t <= tmax
}

func primAnyHitAccel(p *Primitive, ar *RayAccel, tmin, tmax float64) bool {
	if p.Kind == PrimSphere {
		return sphereAnyHitAccel(p, ar, tmin, tmax)
	}
	return triAnyHit(p, ar.Ray, tmin, tmax)
}

type traversalEntry struct {
	node int
	near float64
}

func (s *Scene) Hit(r Ray, tmin, tmax float64) (Hit, bool) {
	if len(s.Nodes) == 0 {
		return Hit{}, false
	}
	ar := prepareRay(r)
	rootOK, rootNear := s.Nodes[0].Box.HitAccel(&ar, tmin, tmax)
	if !rootOK {
		return Hit{}, false
	}
	stack := [16]traversalEntry{}
	sp := 1
	stack[0] = traversalEntry{0, rootNear}
	closest := tmax
	var best Hit
	found := false
	for sp > 0 {
		sp--
		entry := stack[sp]
		ni := entry.node
		if entry.near >= closest {
			continue
		}
		n := &s.Nodes[ni]
		if n.Count > 0 {
			for i := 0; i < n.Count; i++ {
				pi := s.Order[n.Start+i]
				if h, hit := primHitAccel(&s.Primitives[pi], &ar, tmin, closest); hit {
					h.Primitive = pi
					closest, best, found = h.T, h, true
				}
			}
			continue
		}
		leftOK, leftNear := s.Nodes[n.Left].Box.HitAccel(&ar, tmin, closest)
		rightOK, rightNear := s.Nodes[n.Right].Box.HitAccel(&ar, tmin, closest)
		if !leftOK && !rightOK {
			continue
		}
		if sp+2 > len(stack) {
			if leftOK {
				if h, hit := s.hitNodeRecursiveAccel(n.Left, &ar, tmin, closest); hit {
					closest, best, found = h.T, h, true
				}
			}
			if rightOK {
				if h, hit := s.hitNodeRecursiveAccel(n.Right, &ar, tmin, closest); hit {
					closest, best, found = h.T, h, true
				}
			}
			continue
		}
		if leftOK && rightOK {
			if leftNear <= rightNear {
				stack[sp] = traversalEntry{n.Right, rightNear}
				sp++
				stack[sp] = traversalEntry{n.Left, leftNear}
				sp++
			} else {
				stack[sp] = traversalEntry{n.Left, leftNear}
				sp++
				stack[sp] = traversalEntry{n.Right, rightNear}
				sp++
			}
		} else if leftOK {
			stack[sp] = traversalEntry{n.Left, leftNear}
			sp++
		} else {
			stack[sp] = traversalEntry{n.Right, rightNear}
			sp++
		}
	}
	return best, found
}

func (s *Scene) Occluded(r Ray, tmin, tmax float64) bool {
	if len(s.Nodes) == 0 {
		return false
	}
	ar := prepareRay(r)
	if ok, _ := s.Nodes[0].Box.HitAccel(&ar, tmin, tmax); !ok {
		return false
	}
	stack := [16]int{0}
	sp := 1
	for sp > 0 {
		sp--
		ni := stack[sp]
		n := &s.Nodes[ni]
		if n.Count > 0 {
			for i := 0; i < n.Count; i++ {
				pi := s.Order[n.Start+i]
				if primAnyHitAccel(&s.Primitives[pi], &ar, tmin, tmax) {
					return true
				}
			}
			continue
		}
		leftOK, _ := s.Nodes[n.Left].Box.HitAccel(&ar, tmin, tmax)
		rightOK, _ := s.Nodes[n.Right].Box.HitAccel(&ar, tmin, tmax)
		if sp+2 > len(stack) {
			if leftOK && s.occludedNodeRecursive(n.Left, &ar, tmin, tmax) {
				return true
			}
			if rightOK && s.occludedNodeRecursive(n.Right, &ar, tmin, tmax) {
				return true
			}
			continue
		}
		if leftOK {
			stack[sp] = n.Left
			sp++
		}
		if rightOK {
			stack[sp] = n.Right
			sp++
		}
	}
	return false
}

func (s *Scene) occludedNodeRecursive(ni int, ar *RayAccel, tmin, tmax float64) bool {
	n := &s.Nodes[ni]
	if ok, _ := n.Box.HitAccel(ar, tmin, tmax); !ok {
		return false
	}
	if n.Count > 0 {
		for i := 0; i < n.Count; i++ {
			pi := s.Order[n.Start+i]
			if primAnyHitAccel(&s.Primitives[pi], ar, tmin, tmax) {
				return true
			}
		}
		return false
	}
	return s.occludedNodeRecursive(n.Left, ar, tmin, tmax) || s.occludedNodeRecursive(n.Right, ar, tmin, tmax)
}

func (s *Scene) hitNodeRecursiveAccel(ni int, ar *RayAccel, tmin, tmax float64) (Hit, bool) {
	n := &s.Nodes[ni]
	if ok, _ := n.Box.HitAccel(ar, tmin, tmax); !ok {
		return Hit{}, false
	}
	closest := tmax
	var best Hit
	found := false
	if n.Count > 0 {
		for i := 0; i < n.Count; i++ {
			pi := s.Order[n.Start+i]
			if h, hit := primHitAccel(&s.Primitives[pi], ar, tmin, closest); hit {
				h.Primitive = pi
				closest, best, found = h.T, h, true
			}
		}
		return best, found
	}
	if h, hit := s.hitNodeRecursiveAccel(n.Left, ar, tmin, closest); hit {
		closest, best, found = h.T, h, true
	}
	if h, hit := s.hitNodeRecursiveAccel(n.Right, ar, tmin, closest); hit {
		best, found = h, true
	}
	return best, found
}

type Camera struct {
	Position, Target, Up Vec3
	FOV                  float64
}

func DefaultCamera() Camera { return Camera{V(6, 3.2, 5.5), V(0, 0, -4), V(0, 1, 0), 40} }
func (c Camera) basis(aspect float64) (origin, lower, horiz, vert Vec3) {
	theta := c.FOV * math.Pi / 180
	h := math.Tan(theta / 2)
	vh := 2 * h
	vw := aspect * vh
	w := c.Position.Sub(c.Target).Unit()
	u := c.Up.Cross(w).Unit()
	v := w.Cross(u)
	origin = c.Position
	horiz = u.Mul(vw)
	vert = v.Mul(vh)
	lower = origin.Sub(horiz.Mul(.5)).Sub(vert.Mul(.5)).Sub(w)
	return
}
func (c *Camera) Orbit(yaw, pitch float64) {
	off := c.Position.Sub(c.Target)
	r := math.Max(.1, off.Len())
	y := math.Atan2(off.X, off.Z) + yaw*math.Pi/180
	p := math.Asin(clamp(off.Y/r, -.999, .999)) + pitch*math.Pi/180
	p = clamp(p, -1.45, 1.45)
	c.Position = c.Target.Add(V(r*math.Cos(p)*math.Sin(y), r*math.Sin(p), r*math.Cos(p)*math.Cos(y)))
}
func (c *Camera) Dolly(amount float64) {
	d := c.Position.Sub(c.Target)
	l := math.Max(.2, d.Len()+amount)
	c.Position = c.Target.Add(d.Unit().Mul(l))
}
func (c *Camera) Pan(dx, dy float64) {
	f := c.Target.Sub(c.Position).Unit()
	right := f.Cross(c.Up).Unit()
	up := right.Cross(f).Unit()
	delta := right.Mul(dx).Add(up.Mul(dy))
	c.Position = c.Position.Add(delta)
	c.Target = c.Target.Add(delta)
}

type BackendMode string

const (
	BackendAuto BackendMode = "Auto"
	BackendCPU  BackendMode = "CPU"
	BackendGPU  BackendMode = "OpenCL GPU"
)

type UpscaleMode string

const (
	UpscaleOff          UpscaleMode = "Off"
	UpscaleUltraQuality UpscaleMode = "Ultra Quality"
	UpscaleBalanced     UpscaleMode = "Balanced"
	UpscalePerformance  UpscaleMode = "Performance"
)

type IntegratorMode string

const (
	IntegratorPath      IntegratorMode = "Path Tracing"
	IntegratorRecursive IntegratorMode = "Recursive Ray Tracing"
	IntegratorPhoton    IntegratorMode = "Photon Mapping"
)

type DebugViewMode int

const (
	DebugBeauty DebugViewMode = iota
	DebugAlbedo
	DebugNormal
	DebugDepth
	DebugAO
	DebugHeat
)

type SamplingMode string

const (
	SamplerRandom SamplingMode = "Random"
	SamplerHalton SamplingMode = "Halton"
	SamplerR2     SamplingMode = "R2"
)

type RenderSettings struct {
	Width, Height, SPP, Bounces int
	Denoise                     bool
	Quality                     string
	Backend                     BackendMode
	Upscale                     UpscaleMode
	Integrator                  IntegratorMode
	Exposure                    float64
	Gamma                       float64
	FXAA                        int
	TAA                         int
	TSAA                        int
	TXAA                        int
	HBAO                        int
	HBAOPlus                    int
	Bloom                       int
	HDR                         int
	MSAA                        int
	PhotonMapping               int
	DebugView                   DebugViewMode
	TileSize                    int
	CPUWorkers                  int
	AdaptiveSampling            bool
	AdaptiveThreshold           float64
	Sampler                     SamplingMode
	MIS                         bool
	PowerLightSampling          bool
	FireflyClamp                float64
	RRDepth                     int
	TileOrder                   string
	PublishHz                   int
	ProgressiveUpdates          bool
	BVHMode                     string
	BVHBins                     int
	BVHLeafSize                 int
}

func Quality(name string) RenderSettings {
	base := RenderSettings{Backend: BackendAuto, Upscale: UpscaleOff, Integrator: IntegratorPath, Exposure: 0.0, Gamma: 2.2, FXAA: 1, TAA: 1, TSAA: 0, TXAA: 0, HBAO: 0, HBAOPlus: 0, Bloom: 1, HDR: 2, MSAA: 1, PhotonMapping: 0, DebugView: DebugBeauty, TileSize: 16, CPUWorkers: 0, AdaptiveSampling: true, AdaptiveThreshold: 0.0006, Sampler: SamplerHalton, MIS: true, PowerLightSampling: true, FireflyClamp: 24, RRDepth: 4, TileOrder: "Center", PublishHz: 30, ProgressiveUpdates: true, BVHMode: "SAH", BVHBins: 16, BVHLeafSize: 4}
	switch name {
	case "Draft":
		base.Width, base.Height, base.SPP, base.Bounces, base.Denoise, base.Quality = 640, 360, 2, 4, false, name
		base.FXAA, base.TAA, base.Bloom, base.HDR = 1, 0, 0, 1
		base.TileSize = 32
		base.AdaptiveThreshold = 0.002
		base.FireflyClamp = 12
	case "Balanced":
		base.Width, base.Height, base.SPP, base.Bounces, base.Denoise, base.Quality = 1280, 720, 16, 8, true, name
		base.FXAA, base.TAA, base.HBAO, base.Bloom, base.HDR, base.MSAA = 2, 2, 1, 1, 2, 1
		base.PhotonMapping = 1
		base.AdaptiveThreshold = 0.0008
	case "High":
		base.Width, base.Height, base.SPP, base.Bounces, base.Denoise, base.Quality = 1600, 900, 48, 10, true, name
		base.FXAA, base.TAA, base.TSAA, base.HBAOPlus, base.Bloom, base.HDR, base.MSAA = 2, 2, 1, 2, 2, 3, 2
		base.PhotonMapping = 2
		base.AdaptiveThreshold = 0.00035
		base.FireflyClamp = 32
	case "Ultra":
		base.Width, base.Height, base.SPP, base.Bounces, base.Denoise, base.Quality = 1920, 1080, 96, 12, true, name
		base.FXAA, base.TAA, base.TSAA, base.TXAA, base.HBAOPlus, base.Bloom, base.HDR, base.MSAA = 3, 3, 2, 1, 3, 3, 3, 2
		base.PhotonMapping = 3
		base.AdaptiveThreshold = 0.00018
		base.FireflyClamp = 48
	case "Ultra Realism":
		// Physically-oriented final preset: native resolution, deep paths, no fake AO,
		// no post-AA blur, tight convergence, MIS and low-discrepancy sampling.
		base.Width, base.Height, base.SPP, base.Bounces, base.Denoise, base.Quality = 2560, 1440, 256, 16, true, name
		base.Integrator = IntegratorPath
		base.Upscale = UpscaleOff
		base.FXAA, base.TAA, base.TSAA, base.TXAA, base.MSAA = 0, 1, 0, 0, 1
		base.HBAO, base.HBAOPlus = 0, 0
		base.Bloom, base.HDR = 1, 3
		base.PhotonMapping = 0
		base.Sampler = SamplerHalton
		base.MIS = true
		base.PowerLightSampling = true
		base.FireflyClamp = 0
		base.RRDepth = 6
		base.AdaptiveSampling = true
		base.AdaptiveThreshold = 0.00008
		base.TileSize = 8
		base.TileOrder = "Center"
		base.ProgressiveUpdates = false
		base.BVHMode, base.BVHBins, base.BVHLeafSize = "SAH", 32, 4
	default:
		base.Width, base.Height, base.SPP, base.Bounces, base.Denoise, base.Quality = 960, 540, 8, 6, true, "Preview"
		base.FXAA, base.TAA, base.HDR = 1, 1, 2
		base.TileSize = 24
	}
	return base
}

func UltraRealismSettings(backend BackendMode) RenderSettings {
	q := Quality("Ultra Realism")
	q.Backend = backend
	if backend == BackendGPU {
		// Keep explicit OpenCL selection on-device. The portable GPU kernel is the
		// baseline path tracer and does not yet implement the CPU MIS/adaptive stack.
		q.Sampler = SamplerRandom
		q.MIS = false
		q.PowerLightSampling = false
		q.FireflyClamp = 0
		q.AdaptiveSampling = false
		q.ProgressiveUpdates = false
		q.TAA = 1
		q.Denoise = true
	}
	return q
}

func (s *Scene) Bounds() AABB {
	box := emptyAABB()
	for _, p := range s.Primitives {
		box = box.ExpandBox(p.Box)
	}
	if math.IsInf(box.Min.X, 1) {
		return AABB{V(-1, -1, -1), V(1, 1, 1)}
	}
	return box
}

func photonBudget(level int) int {
	switch level {
	case 1:
		return 512
	case 2:
		return 2048
	case 3:
		return 8192
	case 4:
		return 16384
	default:
		return 0
	}
}

func photonRadius(scene *Scene, level int) float64 {
	box := scene.Bounds()
	diag := box.Max.Sub(box.Min).Len()
	if diag < 1e-3 {
		diag = 10
	}
	base := diag * 0.03
	switch level {
	case 1:
		return base * 1.8
	case 2:
		return base * 1.2
	case 3:
		return base * 0.85
	case 4:
		return base * 0.6
	default:
		return base
	}
}

func cpuWorkerCount(settings RenderSettings) int {
	if settings.CPUWorkers > 0 {
		return settings.CPUWorkers
	}
	workers := runtime.GOMAXPROCS(0)
	if workers < 1 {
		workers = 1
	}
	return workers
}

func parallelFor(n, workers int, fn func(start, end int)) {
	if n <= 0 {
		return
	}
	workers = minInt(iMax(workers, 1), n)
	if workers <= 1 || n < 64 {
		fn(0, n)
		return
	}
	var wg sync.WaitGroup
	chunk := (n + workers - 1) / workers
	for wi := 0; wi < workers; wi++ {
		start := wi * chunk
		end := minInt(start+chunk, n)
		if start >= end {
			break
		}
		wg.Add(1)
		go func(a, b int) {
			defer wg.Done()
			fn(a, b)
		}(start, end)
	}
	wg.Wait()
}

type RenderState struct {
	mu            sync.RWMutex
	Width, Height int
	Pixels        []uint32
	Linear        []Vec3
	Guides        []Guide
	Progress      float64
	Rendering     bool
	Status        string
	Seconds       float64
	Rays          int64
	FrameSerial   uint64
	BackendUsed   string
	DeviceName    string
	generation    atomic.Uint64
	currentCancel *atomic.Bool

	historyLinear []Vec3
	historyGuides []Guide
	historyW      int
	historyH      int
	historyCamera Camera
	historyValid  bool
}

func NewRenderState() *RenderState { return &RenderState{Status: "Ready"} }
func (r *RenderState) Snapshot() (w, h int, pix []uint32, progress float64, rendering bool, status string, seconds float64, rays int64) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	pix = append([]uint32(nil), r.Pixels...)
	return r.Width, r.Height, pix, r.Progress, r.Rendering, r.Status, r.Seconds, r.Rays
}
func (r *RenderState) CancelRender() {
	r.mu.RLock()
	cancel := r.currentCancel
	r.mu.RUnlock()
	if cancel != nil {
		cancel.Store(true)
	}
}

func randInUnitSphere(rng *rand.Rand) Vec3 {
	for {
		p := V(rng.Float64()*2-1, rng.Float64()*2-1, rng.Float64()*2-1)
		if l2 := p.Len2(); l2 < 1 && l2 > 1e-18 {
			return p
		}
	}
}

// randUnitVector directly samples a uniform direction with two random values.
// It replaces rejection-sample + normalize in places that only need direction.
func randUnitVector(rng *rand.Rand) Vec3 {
	// Marsaglia sphere sampler: no trigonometry and uniform over the sphere.
	for {
		u := rng.Float64()*2 - 1
		v := rng.Float64()*2 - 1
		s := u*u + v*v
		if s >= 1 || s <= 1e-18 {
			continue
		}
		f := 2 * math.Sqrt(1-s)
		return V(u*f, v*f, 1-2*s)
	}
}

// randCosHemisphere uses uniform-disk rejection + Malley's projection.
// This preserves the cosine-weighted hemisphere distribution while avoiding
// the sin/cos pair that used to execute at every diffuse bounce.
func randCosHemisphere(n Vec3, rng *rand.Rand) Vec3 {
	var x, y, s float64
	for {
		x = rng.Float64()*2 - 1
		y = rng.Float64()*2 - 1
		s = x*x + y*y
		if s < 1 {
			break
		}
	}
	z := math.Sqrt(math.Max(0, 1-s))

	sign := 1.0
	if n.Z < 0 {
		sign = -1
	}
	a := -1.0 / (sign + n.Z)
	b := n.X * n.Y * a
	t := V(1+sign*n.X*n.X*a, sign*b, -sign*n.X)
	bt := V(b, sign+n.Y*n.Y*a, -n.Y)
	return t.Mul(x).Add(bt.Mul(y)).Add(n.Mul(z))
}
func reflect(v, n Vec3) Vec3 { return v.Sub(n.Mul(2 * v.Dot(n))) }
func refract(uv, n Vec3, eta float64) Vec3 {
	ct := math.Min(uv.Mul(-1).Dot(n), 1)
	perp := uv.Add(n.Mul(ct)).Mul(eta)
	par := n.Mul(-math.Sqrt(math.Abs(1 - perp.Len2())))
	return perp.Add(par)
}
func schlick(cosine, ref float64) float64 {
	r0 := (1 - ref) / (1 + ref)
	r0 *= r0
	x := 1 - cosine
	x2 := x * x
	x5 := x2 * x2 * x
	return r0 + (1-r0)*x5
}
func background(d Vec3) Vec3 {
	u := d.Unit()
	t := .5 * (u.Y + 1)
	return V(.06, .08, .12).Mul(1 - t).Add(V(.32, .48, .72).Mul(t))
}

func visible(scene *Scene, from, to Vec3) bool {
	d := to.Sub(from)
	dist2 := d.Len2()
	if dist2 < 1e-12 {
		return true
	}
	dist := math.Sqrt(dist2)
	return !scene.Occluded(Ray{from, d.Mul(1 / dist)}, 1e-4, dist-1e-4)
}
func lightWeight(scene *Scene, primIndex int) float64 {
	if primIndex < 0 || primIndex >= len(scene.Primitives) {
		return 0
	}
	p := scene.Primitives[primIndex]
	if p.Kind != PrimSphere || p.Material < 0 || p.Material >= len(scene.Materials) {
		return 0
	}
	m := scene.Materials[p.Material]
	if m.Kind != MatEmissive {
		return 0
	}
	area := 4 * math.Pi * p.Radius * p.Radius
	return math.Max(1e-9, luminance(m.Emission)*area)
}

func lightSelectionPDF(scene *Scene, primIndex int, powerWeighted bool) float64 {
	if len(scene.LightSpheres) == 0 {
		return 0
	}
	if !powerWeighted {
		return 1 / float64(len(scene.LightSpheres))
	}
	if primIndex >= 0 && primIndex < len(scene.LightPDFByPrim) {
		return scene.LightPDFByPrim[primIndex]
	}
	return 0
}

func chooseLight(scene *Scene, rng *rand.Rand, powerWeighted bool) (int, float64) {
	if len(scene.LightSpheres) == 0 {
		return -1, 0
	}
	if !powerWeighted || len(scene.LightCDF) != len(scene.LightSpheres) {
		idx := rng.Intn(len(scene.LightSpheres))
		return scene.LightSpheres[idx], 1 / float64(len(scene.LightSpheres))
	}
	r := rng.Float64()
	idx := sort.Search(len(scene.LightCDF), func(i int) bool { return scene.LightCDF[i] >= r })
	if idx >= len(scene.LightSpheres) {
		idx = len(scene.LightSpheres) - 1
	}
	pi := scene.LightSpheres[idx]
	pdf := 0.0
	if pi >= 0 && pi < len(scene.LightPDFByPrim) {
		pdf = scene.LightPDFByPrim[pi]
	}
	if pdf <= 0 {
		pdf = 1 / float64(len(scene.LightSpheres))
	}
	return pi, pdf
}

func powerHeuristic(a, b float64) float64 {
	a2 := a * a
	b2 := b * b
	if a2+b2 <= 1e-20 {
		return 0
	}
	return a2 / (a2 + b2)
}

func lightDirectionalPDF(scene *Scene, primIndex int, from, lightPoint, lightNormal Vec3, powerWeighted bool) float64 {
	if primIndex < 0 || primIndex >= len(scene.Primitives) {
		return 0
	}
	p := scene.Primitives[primIndex]
	if p.Kind != PrimSphere || p.Radius <= 0 {
		return 0
	}
	to := lightPoint.Sub(from)
	dist2 := to.Len2()
	if dist2 <= 1e-12 {
		return 0
	}
	ld := to.Unit()
	cosL := math.Max(0, lightNormal.Dot(ld.Mul(-1)))
	if cosL <= 1e-12 {
		return 0
	}
	area := 4 * math.Pi * p.Radius * p.Radius
	selectPDF := lightSelectionPDF(scene, primIndex, powerWeighted)
	return selectPDF * dist2 / (cosL * area)
}

func directLight(scene *Scene, h *Hit, hitP Vec3, mat *Material, rng *rand.Rand, settings *RenderSettings) Vec3 {
	if mat.Kind != MatDiffuse || len(scene.LightSpheres) == 0 {
		return V(0, 0, 0)
	}
	li, selectPDF := chooseLight(scene, rng, settings.PowerLightSampling)
	if li < 0 || selectPDF <= 0 {
		return V(0, 0, 0)
	}
	lp := scene.Primitives[li]
	lm := scene.Materials[lp.Material]
	ln := randUnitVector(rng)
	q := lp.Center.Add(ln.Mul(lp.Radius))
	to := q.Sub(hitP)
	dist2 := to.Len2()
	if dist2 < 1e-8 {
		return V(0, 0, 0)
	}
	ld := to.Unit()
	nd := math.Max(0, h.N.Dot(ld))
	cosL := math.Max(0, ln.Mul(-1).Dot(ld))
	if nd <= 0 || cosL <= 0 {
		return V(0, 0, 0)
	}
	if !visible(scene, hitP.Add(h.N.Mul(2e-4)), q) {
		return V(0, 0, 0)
	}
	area := 4 * math.Pi * lp.Radius * lp.Radius
	lightPDF := selectPDF * dist2 / (cosL * area)
	if lightPDF <= 0 {
		return V(0, 0, 0)
	}
	bsdfPDF := nd / math.Pi
	w := 1.0
	if settings.MIS {
		w = powerHeuristic(lightPDF, bsdfPDF)
	}
	f := mat.Color.Mul(1 / math.Pi)
	return f.Had(lm.Emission).Mul(nd * w / lightPDF)
}

func materialPreviewColor(m *Material) Vec3 {
	if m.Kind == MatEmissive {
		return V(clamp(m.Emission.X/8, 0, 1), clamp(m.Emission.Y/8, 0, 1), clamp(m.Emission.Z/8, 0, 1))
	}
	if m.Color.Len2() > 0 {
		return m.Color
	}
	return V(0.8, 0.8, 0.8)
}

type RayCounter struct {
	n int64
}

func (c *RayCounter) Add(v int64) { c.n += v }
func (c *RayCounter) Load() int64 { return c.n }

func trace(scene *Scene, ray Ray, settings *RenderSettings, rng *rand.Rand, rays *RayCounter, guide *Guide) Vec3 {
	throughput := V(1, 1, 1)
	radiance := V(0, 0, 0)
	prevWasDiffuse := false
	prevBSDFPDF := 0.0
	prevPoint := V(0, 0, 0)
	for bounce := 0; bounce < settings.Bounces; bounce++ {
		rays.Add(1)
		h, ok := scene.Hit(ray, 1e-4, 1e30)
		if !ok {
			return radiance.Add(throughput.Had(background(ray.D)))
		}
		if h.Material < 0 || h.Material >= len(scene.Materials) {
			return radiance
		}
		m := &scene.Materials[h.Material]
		p := ray.At(h.T)
		if bounce == 0 && guide != nil {
			*guide = Guide{P: p, N: h.N, Albedo: materialPreviewColor(m), Depth: h.T, Valid: true}
		}
		if m.Kind == MatEmissive {
			w := 1.0
			if settings.MIS && prevWasDiffuse && h.Primitive >= 0 {
				lightPDF := lightDirectionalPDF(scene, h.Primitive, prevPoint, p, h.N, settings.PowerLightSampling)
				w = powerHeuristic(prevBSDFPDF, lightPDF)
			}
			return radiance.Add(throughput.Had(m.Emission).Mul(w))
		}
		if m.Kind == MatDiffuse {
			radiance = radiance.Add(throughput.Had(directLight(scene, &h, p, m, rng, settings)))
			throughput = throughput.Had(m.Color)
			d := randCosHemisphere(h.N, rng)
			prevWasDiffuse = true
			prevBSDFPDF = math.Max(0, h.N.Dot(d)) / math.Pi
			prevPoint = p
			ray = Ray{p.Add(h.N.Mul(2e-4)), d}
		} else if m.Kind == MatMetal {
			d := reflect(ray.D.Unit(), h.N).Add(randInUnitSphere(rng).Mul(clamp(m.Roughness, 0, 1))).Unit()
			if d.Dot(h.N) <= 0 {
				return radiance
			}
			prevWasDiffuse = false
			throughput = throughput.Had(m.Color)
			ray = Ray{p.Add(h.N.Mul(2e-4)), d}
		} else if m.Kind == MatGlass {
			eta := m.IOR
			if eta <= 1 {
				eta = 1.5
			}
			ratio := eta
			if h.Front {
				ratio = 1 / eta
			}
			unit := ray.D.Unit()
			cosTheta := math.Min(unit.Mul(-1).Dot(h.N), 1)
			sinTheta := math.Sqrt(math.Max(0, 1-cosTheta*cosTheta))
			var d Vec3
			if ratio*sinTheta > 1 || schlick(cosTheta, ratio) > rng.Float64() {
				d = reflect(unit, h.N)
			} else {
				d = refract(unit, h.N, ratio)
			}
			prevWasDiffuse = false
			throughput = throughput.Had(m.Color)
			ray = Ray{p.Add(d.Mul(2e-4)), d}
		}
		rrDepth := settings.RRDepth
		if rrDepth < 1 {
			rrDepth = 4
		}
		if bounce >= rrDepth {
			p := clamp(throughput.MaxComp(), .08, .95)
			if rng.Float64() > p {
				return radiance
			}
			throughput = throughput.Mul(1 / p)
		}
	}
	return radiance
}

func traceRecursive(scene *Scene, ray Ray, settings *RenderSettings, rng *rand.Rand, rays *RayCounter, guide *Guide) Vec3 {
	return traceRecursiveInner(scene, ray, settings.Bounces, settings, rng, rays, guide, 0)
}

func traceRecursiveInner(scene *Scene, ray Ray, depth int, settings *RenderSettings, rng *rand.Rand, rays *RayCounter, guide *Guide, bounce int) Vec3 {
	if depth <= 0 {
		return V(0, 0, 0)
	}
	rays.Add(1)
	h, ok := scene.Hit(ray, 1e-4, 1e30)
	if !ok {
		return background(ray.D)
	}
	if h.Material < 0 || h.Material >= len(scene.Materials) {
		return V(0, 0, 0)
	}
	m := &scene.Materials[h.Material]
	p := ray.At(h.T)
	if bounce == 0 && guide != nil {
		*guide = Guide{P: p, N: h.N, Albedo: materialPreviewColor(m), Depth: h.T, Valid: true}
	}
	if m.Kind == MatEmissive {
		return m.Emission
	}
	if m.Kind == MatDiffuse {
		bounceColor := traceRecursiveInner(scene, Ray{p.Add(h.N.Mul(2e-4)), randCosHemisphere(h.N, rng)}, depth-1, settings, rng, rays, nil, bounce+1)
		result := directLight(scene, &h, p, m, rng, settings).Add(m.Color.Had(bounceColor))
		if depth < 8 {
			p := clamp(m.Color.MaxComp(), .1, .95)
			return result.Mul(1 / p)
		}
		return result
	}
	if m.Kind == MatMetal {
		d := reflect(ray.D.Unit(), h.N).Add(randInUnitSphere(rng).Mul(clamp(m.Roughness, 0, 1))).Unit()
		if d.Dot(h.N) <= 0 {
			return V(0, 0, 0)
		}
		return m.Color.Had(traceRecursiveInner(scene, Ray{p.Add(h.N.Mul(2e-4)), d}, depth-1, settings, rng, rays, nil, bounce+1))
	}
	eta := m.IOR
	if eta <= 1 {
		eta = 1.5
	}
	ratio := eta
	if h.Front {
		ratio = 1 / eta
	}
	unit := ray.D.Unit()
	cosTheta := math.Min(unit.Mul(-1).Dot(h.N), 1)
	sinTheta := math.Sqrt(math.Max(0, 1-cosTheta*cosTheta))
	var d Vec3
	if ratio*sinTheta > 1 || schlick(cosTheta, ratio) > rng.Float64() {
		d = reflect(unit, h.N)
	} else {
		d = refract(unit, h.N, ratio)
	}
	return m.Color.Had(traceRecursiveInner(scene, Ray{p.Add(d.Mul(2e-4)), d}, depth-1, settings, rng, rays, nil, bounce+1))
}

type Photon struct {
	P, N Vec3
	Flux Vec3
}

type PhotonMap struct {
	Photons []Photon
	Grid    map[uint64][]int
	Cell    float64
	Radius  float64
}

func photonCellKey(x, y, z int) uint64 {
	h := int64(x)*73856093 ^ int64(y)*19349663 ^ int64(z)*83492791
	return uint64(h)
}

func buildPhotonMap(scene *Scene, settings RenderSettings, cancel *atomic.Bool, rays *RayCounter, seed int64) PhotonMap {
	budget := photonBudget(settings.PhotonMapping)
	if budget <= 0 || len(scene.LightSpheres) == 0 {
		return PhotonMap{}
	}
	radius := photonRadius(scene, settings.PhotonMapping)
	if radius <= 1e-4 {
		radius = 0.25
	}
	pm := PhotonMap{Photons: make([]Photon, 0, budget), Grid: make(map[uint64][]int, iMax(16, budget/4)), Cell: radius, Radius: radius}
	rng := rand.New(rand.NewSource(seed))
	maxBounces := iMax(2, settings.Bounces)
	attempts := budget * 6
	for emitted := 0; emitted < attempts && len(pm.Photons) < budget; emitted++ {
		if cancel != nil && cancel.Load() {
			break
		}
		li := scene.LightSpheres[rng.Intn(len(scene.LightSpheres))]
		lp := scene.Primitives[li]
		lm := scene.Materials[lp.Material]
		surf := randUnitVector(rng)
		origin := lp.Center.Add(surf.Mul(lp.Radius + 2e-4))
		dir := randCosHemisphere(surf, rng)
		flux := lm.Emission.Mul((4 * math.Pi * lp.Radius * lp.Radius) / float64(budget))
		ray := Ray{origin, dir}
		throughput := flux
		for bounce := 0; bounce < maxBounces; bounce++ {
			rays.Add(1)
			h, ok := scene.Hit(ray, 1e-4, 1e30)
			if !ok || h.Material < 0 || h.Material >= len(scene.Materials) {
				break
			}
			m := &scene.Materials[h.Material]
			p := ray.At(h.T)
			if m.Kind == MatEmissive {
				break
			}
			if m.Kind == MatDiffuse {
				pm.Photons = append(pm.Photons, Photon{P: p, N: h.N, Flux: throughput.Had(m.Color)})
				throughput = throughput.Had(m.Color)
				ray = Ray{p.Add(h.N.Mul(2e-4)), randCosHemisphere(h.N, rng)}
			} else if m.Kind == MatMetal {
				d := reflect(ray.D.Unit(), h.N).Add(randInUnitSphere(rng).Mul(clamp(m.Roughness, 0, 1))).Unit()
				if d.Dot(h.N) <= 0 {
					break
				}
				throughput = throughput.Had(m.Color)
				ray = Ray{p.Add(h.N.Mul(2e-4)), d}
			} else {
				eta := m.IOR
				if eta <= 1 {
					eta = 1.5
				}
				ratio := eta
				if h.Front {
					ratio = 1 / eta
				}
				unit := ray.D.Unit()
				cosTheta := math.Min(unit.Mul(-1).Dot(h.N), 1)
				sinTheta := math.Sqrt(math.Max(0, 1-cosTheta*cosTheta))
				var d Vec3
				if ratio*sinTheta > 1 || schlick(cosTheta, ratio) > rng.Float64() {
					d = reflect(unit, h.N)
				} else {
					d = refract(unit, h.N, ratio)
				}
				throughput = throughput.Had(m.Color)
				ray = Ray{p.Add(d.Mul(2e-4)), d}
			}
			if bounce >= 2 {
				p := clamp(throughput.MaxComp(), .1, .95)
				if rng.Float64() > p {
					break
				}
				throughput = throughput.Mul(1 / p)
			}
		}
	}
	for i, ph := range pm.Photons {
		cx := int(math.Floor(ph.P.X / pm.Cell))
		cy := int(math.Floor(ph.P.Y / pm.Cell))
		cz := int(math.Floor(ph.P.Z / pm.Cell))
		key := photonCellKey(cx, cy, cz)
		pm.Grid[key] = append(pm.Grid[key], i)
	}
	return pm
}

func (pm PhotonMap) gather(p, n Vec3, maxPhotons int) Vec3 {
	if len(pm.Photons) == 0 || pm.Cell <= 0 {
		return V(0, 0, 0)
	}
	cx := int(math.Floor(p.X / pm.Cell))
	cy := int(math.Floor(p.Y / pm.Cell))
	cz := int(math.Floor(p.Z / pm.Cell))
	r2 := pm.Radius * pm.Radius
	sum := V(0, 0, 0)
	count := 0
	for dz := -1; dz <= 1; dz++ {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				for _, idx := range pm.Grid[photonCellKey(cx+dx, cy+dy, cz+dz)] {
					ph := pm.Photons[idx]
					d := ph.P.Sub(p)
					nd := ph.N.Dot(n)
					if d.Len2() > r2 || nd <= 0.05 {
						continue
					}
					sum = sum.Add(ph.Flux.Mul(nd))
					count++
					if maxPhotons > 0 && count >= maxPhotons {
						return sum.Mul(1 / (math.Pi * pm.Radius * pm.Radius * float64(count)))
					}
				}
			}
		}
	}
	if count == 0 {
		return V(0, 0, 0)
	}
	return sum.Mul(1 / (math.Pi * pm.Radius * pm.Radius * float64(count)))
}

func tracePhotonMapped(scene *Scene, ray Ray, settings *RenderSettings, rng *rand.Rand, rays *RayCounter, guide *Guide, pm PhotonMap) Vec3 {
	throughput := V(1, 1, 1)
	radiance := V(0, 0, 0)
	for bounce := 0; bounce < settings.Bounces; bounce++ {
		rays.Add(1)
		h, ok := scene.Hit(ray, 1e-4, 1e30)
		if !ok {
			return radiance.Add(throughput.Had(background(ray.D)))
		}
		if h.Material < 0 || h.Material >= len(scene.Materials) {
			return radiance
		}
		m := &scene.Materials[h.Material]
		p := ray.At(h.T)
		if bounce == 0 && guide != nil {
			*guide = Guide{P: p, N: h.N, Albedo: materialPreviewColor(m), Depth: h.T, Valid: true}
		}
		if m.Kind == MatEmissive {
			return radiance.Add(throughput.Had(m.Emission))
		}
		if m.Kind == MatDiffuse {
			phot := pm.gather(p, h.N, 64)
			radiance = radiance.Add(throughput.Had(directLight(scene, &h, p, m, rng, settings).Add(phot)))
			throughput = throughput.Had(m.Color)
			ray = Ray{p.Add(h.N.Mul(2e-4)), randCosHemisphere(h.N, rng)}
		} else if m.Kind == MatMetal {
			d := reflect(ray.D.Unit(), h.N).Add(randInUnitSphere(rng).Mul(clamp(m.Roughness, 0, 1))).Unit()
			if d.Dot(h.N) <= 0 {
				return radiance
			}
			throughput = throughput.Had(m.Color)
			ray = Ray{p.Add(h.N.Mul(2e-4)), d}
		} else {
			eta := m.IOR
			if eta <= 1 {
				eta = 1.5
			}
			ratio := eta
			if h.Front {
				ratio = 1 / eta
			}
			unit := ray.D.Unit()
			cosTheta := math.Min(unit.Mul(-1).Dot(h.N), 1)
			sinTheta := math.Sqrt(math.Max(0, 1-cosTheta*cosTheta))
			var d Vec3
			if ratio*sinTheta > 1 || schlick(cosTheta, ratio) > rng.Float64() {
				d = reflect(unit, h.N)
			} else {
				d = refract(unit, h.N, ratio)
			}
			throughput = throughput.Had(m.Color)
			ray = Ray{p.Add(d.Mul(2e-4)), d}
		}
		rrDepth := settings.RRDepth
		if rrDepth < 1 {
			rrDepth = 4
		}
		if bounce >= rrDepth {
			p := clamp(throughput.MaxComp(), .08, .95)
			if rng.Float64() > p {
				return radiance
			}
			throughput = throughput.Mul(1 / p)
		}
	}
	return radiance
}

func pixelRadiance(scene *Scene, ray Ray, settings *RenderSettings, rng *rand.Rand, rays *RayCounter, guide *Guide, pm PhotonMap) Vec3 {
	switch settings.Integrator {
	case IntegratorRecursive:
		return traceRecursive(scene, ray, settings, rng, rays, guide)
	case IntegratorPhoton:
		return tracePhotonMapped(scene, ray, settings, rng, rays, guide, pm)
	default:
		return trace(scene, ray, settings, rng, rays, guide)
	}
}

func radicalInverse(n uint64, base uint64) float64 {
	if base == 2 {
		return float64(bits.Reverse64(n)) * 5.421010862427522e-20
	}
	invBase := 1.0 / float64(base)
	inv := invBase
	result := 0.0
	for n > 0 {
		digit := n % base
		result += float64(digit) * inv
		n /= base
		inv *= invBase
	}
	return result
}

func fract(x float64) float64 { return x - math.Floor(x) }

func sampleJitter(settings RenderSettings, x, y, sample int, rng interface{ Float64() float64 }) (float64, float64) {
	switch settings.Sampler {
	case SamplerHalton:
		h := uint64((x * 73856093) ^ (y * 19349663))
		n := uint64(sample+1) + (h&1023)*4096
		return radicalInverse(n, 2), radicalInverse(n, 3)
	case SamplerR2:
		const g = 1.324717957244746
		a1 := 1.0 / g
		a2 := 1.0 / (g * g)
		h := float64(((x*73856093)^(y*19349663))&65535) / 65536.0
		return fract(0.5 + h + float64(sample+1)*a1), fract(0.5 + h*0.61803398875 + float64(sample+1)*a2)
	default:
		return rng.Float64(), rng.Float64()
	}
}

type SampleSequence struct {
	X, Y []float64
}

func buildSampleSequence(settings *RenderSettings) *SampleSequence {
	if settings.Sampler == SamplerRandom {
		return nil
	}
	n := effectiveSPP(settings)
	seq := &SampleSequence{X: make([]float64, n), Y: make([]float64, n)}
	if settings.Sampler == SamplerR2 {
		const g = 1.324717957244746
		a1 := 1.0 / g
		a2 := 1.0 / (g * g)
		for i := 0; i < n; i++ {
			seq.X[i] = fract(0.5 + float64(i+1)*a1)
			seq.Y[i] = fract(0.5 + float64(i+1)*a2)
		}
		return seq
	}
	for i := 0; i < n; i++ {
		seq.X[i] = radicalInverse(uint64(i+1), 2)
		seq.Y[i] = radicalInverse(uint64(i+1), 3)
	}
	return seq
}

func pixelSampleRotation(x, y int) (float64, float64) {
	h1 := uint32(x)*0x9e3779b1 ^ uint32(y)*0x85ebca77
	h1 ^= h1 >> 16
	h1 *= 0x7feb352d
	h1 ^= h1 >> 15
	h2 := h1 ^ 0x68bc21eb
	h2 ^= h2 >> 13
	const inv = 1.0 / 4294967296.0
	return float64(h1) * inv, float64(h2) * inv
}

func clampFirefly(c Vec3, limit float64) Vec3 {
	if limit <= 0 {
		return c
	}
	m := c.MaxComp()
	if m <= limit || m <= 0 {
		return c
	}
	return c.Mul(limit / m)
}

func adaptivePixel(scene *Scene, origin, lower, horiz, vert Vec3, x, y, renderW, renderH int, settings *RenderSettings, rng *rand.Rand, rays *RayCounter, pm PhotonMap, guide *Guide, cancel *atomic.Bool, seq *SampleSequence) Vec3 {
	pixelSPP := effectiveSPP(settings)
	sum := V(0, 0, 0)
	sumLum := 0.0
	sumSqLum := 0.0
	minSamples := minInt(8, pixelSPP)
	rotX, rotY := pixelSampleRotation(x, y)
	for sample := 0; sample < pixelSPP; sample++ {
		if cancel != nil && sample&7 == 0 && cancel.Load() {
			if sample == 0 {
				return sum
			}
			return sum.Mul(1 / float64(sample))
		}
		jx, jy := 0.0, 0.0
		if seq != nil && sample < len(seq.X) {
			jx, jy = fract(seq.X[sample]+rotX), fract(seq.Y[sample]+rotY)
		} else {
			jx, jy = rng.Float64(), rng.Float64()
		}
		u := (float64(x) + jx) / float64(iMax(renderW-1, 1))
		v := (float64(renderH-1-y) + jy) / float64(iMax(renderH-1, 1))
		dir := lower.Add(horiz.Mul(u)).Add(vert.Mul(v)).Sub(origin)
		var sampleGuide *Guide
		if sample == 0 {
			sampleGuide = guide
		}
		c := clampFirefly(pixelRadiance(scene, Ray{origin, dir}, settings, rng, rays, sampleGuide, pm), settings.FireflyClamp)
		sum = sum.Add(c)
		lum := luminance(c)
		sumLum += lum
		sumSqLum += lum * lum
		if settings.AdaptiveSampling && sample+1 >= minSamples && (sample+1)%2 == 0 {
			n := float64(sample + 1)
			mean := sumLum / n
			variance := math.Max(0, sumSqLum/n-mean*mean)
			if variance < math.Max(1e-8, settings.AdaptiveThreshold) {
				return sum.Mul(1 / n)
			}
		}
	}
	return sum.Mul(1 / float64(pixelSPP))
}

func makeDebugView(linear []Vec3, guides []Guide, w, h int, settings RenderSettings) []Vec3 {
	if settings.DebugView == DebugBeauty {
		return append([]Vec3(nil), linear...)
	}
	out := make([]Vec3, len(linear))
	switch settings.DebugView {
	case DebugAlbedo:
		for i, g := range guides {
			if g.Valid {
				out[i] = g.Albedo
			}
		}
	case DebugNormal:
		for i, g := range guides {
			if g.Valid {
				out[i] = g.N.Add(V(1, 1, 1)).Mul(0.5)
			}
		}
	case DebugDepth:
		maxDepth := 1.0
		for _, g := range guides {
			if g.Valid && g.Depth > maxDepth {
				maxDepth = g.Depth
			}
		}
		for i, g := range guides {
			if g.Valid {
				v := 1 - clamp(g.Depth/maxDepth, 0, 1)
				out[i] = V(v, v, v)
			}
		}
	case DebugAO:
		base := make([]Vec3, len(linear))
		for i := range base {
			base[i] = V(1, 1, 1)
		}
		level := 2
		plus := false
		if settings.HBAOPlus > 0 {
			level = settings.HBAOPlus
			plus = true
		} else if settings.HBAO > 0 {
			level = settings.HBAO
		}
		out = applyHBAO(base, guides, w, h, iMax(level, 1), plus)
	case DebugHeat:
		maxLum := 1e-6
		for _, c := range linear {
			if l := luminance(c); l > maxLum {
				maxLum = l
			}
		}
		for i, c := range linear {
			t := clamp(luminance(c)/maxLum, 0, 1)
			out[i] = V(clamp(1.5*t, 0, 1), clamp(1.2*(1-math.Abs(t-0.5)*2), 0, 1), clamp(1.5*(1-t), 0, 1))
		}
	default:
		copy(out, linear)
	}
	return out
}

func toneMapHDR(c Vec3, hdr int) Vec3 {
	switch hdr {
	case 0:
		return V(clamp(c.X, 0, 1), clamp(c.Y, 0, 1), clamp(c.Z, 0, 1))
	case 1: // Reinhard
		return V(c.X/(1+c.X), c.Y/(1+c.Y), c.Z/(1+c.Z))
	case 3: // ACES-inspired fit
		aces := func(x float64) float64 {
			a, b, cc, d, e := 2.51, 0.03, 2.43, 0.59, 0.14
			return clamp((x*(a*x+b))/(x*(cc*x+d)+e), 0, 1)
		}
		return V(aces(c.X), aces(c.Y), aces(c.Z))
	default: // filmic shoulder, balanced default
		filmic := func(x float64) float64 {
			x = math.Max(0, x-0.004)
			return clamp((x*(6.2*x+0.5))/(x*(6.2*x+1.7)+0.06), 0, 1)
		}
		return V(filmic(c.X), filmic(c.Y), filmic(c.Z))
	}
}

func toRGBA(c Vec3, exposure, gamma float64, hdr int) uint32 {
	exposureScale := math.Pow(2, exposure)
	c = toneMapHDR(c.Mul(exposureScale), hdr)
	invGamma := 1.0 / math.Max(0.1, gamma)
	c = V(math.Pow(math.Max(0, c.X), invGamma), math.Pow(math.Max(0, c.Y), invGamma), math.Pow(math.Max(0, c.Z), invGamma))
	r := uint32(clamp(c.X, 0, .999) * 256)
	g := uint32(clamp(c.Y, 0, .999) * 256)
	b := uint32(clamp(c.Z, 0, .999) * 256)
	return 0xff000000 | (b << 16) | (g << 8) | r
}

func internalSize(settings RenderSettings) (int, int) {
	scale := 1.0
	switch settings.Upscale {
	case UpscaleUltraQuality:
		scale = 0.77
	case UpscaleBalanced:
		scale = 0.67
	case UpscalePerformance:
		scale = 0.50
	}
	w := iMax(16, int(math.Round(float64(settings.Width)*scale)))
	h := iMax(16, int(math.Round(float64(settings.Height)*scale)))
	return w, h
}

func cubicHermite(a, b, c, d, t float64) float64 {
	ta := -0.5*a + 1.5*b - 1.5*c + 0.5*d
	tb := a - 2.5*b + 2*c - 0.5*d
	tc := -0.5*a + 0.5*c
	td := b
	return ((ta*t+tb)*t+tc)*t + td
}

func sampleClamped(src []Vec3, w, h, x, y int) Vec3 {
	x = minInt(iMax(x, 0), w-1)
	y = minInt(iMax(y, 0), h-1)
	return src[y*w+x]
}

func bicubicUpscale(src []Vec3, sw, sh, dw, dh int) []Vec3 {
	if sw == dw && sh == dh {
		return append([]Vec3(nil), src...)
	}
	out := make([]Vec3, dw*dh)
	if sw <= 0 || sh <= 0 || dw <= 0 || dh <= 0 {
		return out
	}
	parallelFor(dh, runtime.GOMAXPROCS(0), func(ys, ye int) {
		for y := ys; y < ye; y++ {
			fy := 0.0
			if dh > 1 {
				fy = float64(y) * float64(sh-1) / float64(dh-1)
			}
			yi := int(math.Floor(fy))
			ty := fy - float64(yi)
			for x := 0; x < dw; x++ {
				fx := 0.0
				if dw > 1 {
					fx = float64(x) * float64(sw-1) / float64(dw-1)
				}
				xi := int(math.Floor(fx))
				tx := fx - float64(xi)
				var rows [4]Vec3
				for m := -1; m <= 2; m++ {
					p0 := sampleClamped(src, sw, sh, xi-1, yi+m)
					p1 := sampleClamped(src, sw, sh, xi, yi+m)
					p2 := sampleClamped(src, sw, sh, xi+1, yi+m)
					p3 := sampleClamped(src, sw, sh, xi+2, yi+m)
					rows[m+1] = V(
						cubicHermite(p0.X, p1.X, p2.X, p3.X, tx),
						cubicHermite(p0.Y, p1.Y, p2.Y, p3.Y, tx),
						cubicHermite(p0.Z, p1.Z, p2.Z, p3.Z, tx),
					)
				}
				c := V(
					cubicHermite(rows[0].X, rows[1].X, rows[2].X, rows[3].X, ty),
					cubicHermite(rows[0].Y, rows[1].Y, rows[2].Y, rows[3].Y, ty),
					cubicHermite(rows[0].Z, rows[1].Z, rows[2].Z, rows[3].Z, ty),
				)
				out[y*dw+x] = V(math.Max(0, c.X), math.Max(0, c.Y), math.Max(0, c.Z))
			}
		}
	})
	return out
}

func luminance(c Vec3) float64 {
	return 0.2126*c.X + 0.7152*c.Y + 0.0722*c.Z
}

func edgeAwareSharpen(img []Vec3, w, h int, amount float64) []Vec3 {
	if amount <= 0 || w <= 0 || h <= 0 {
		return append([]Vec3(nil), img...)
	}
	out := make([]Vec3, len(img))
	parallelFor(h, runtime.GOMAXPROCS(0), func(ys, ye int) {
		for y := ys; y < ye; y++ {
			for x := 0; x < w; x++ {
				var sum Vec3
				count := 0.0
				center := img[y*w+x]
				centerLum := luminance(center)
				edgeWeight := 0.0
				for oy := -1; oy <= 1; oy++ {
					yy := y + oy
					if yy < 0 || yy >= h {
						continue
					}
					for ox := -1; ox <= 1; ox++ {
						xx := x + ox
						if xx < 0 || xx >= w {
							continue
						}
						neighbor := img[yy*w+xx]
						sum = sum.Add(neighbor)
						count++
						edgeWeight += math.Abs(centerLum - luminance(neighbor))
					}
				}
				blur := sum.Mul(1 / math.Max(count, 1))
				local := amount * clamp(edgeWeight/2.0, 0.35, 1.0)
				c := center.Add(center.Sub(blur).Mul(local))
				out[y*w+x] = V(math.Max(0, c.X), math.Max(0, c.Y), math.Max(0, c.Z))
			}
		}
	})
	return out
}

func upscaleDLSSBasic(src []Vec3, sw, sh, dw, dh int, mode UpscaleMode) []Vec3 {
	out := bicubicUpscale(src, sw, sh, dw, dh)
	switch mode {
	case UpscaleUltraQuality:
		return edgeAwareSharpen(out, dw, dh, 0.10)
	case UpscaleBalanced:
		return edgeAwareSharpen(out, dw, dh, 0.18)
	case UpscalePerformance:
		return edgeAwareSharpen(out, dw, dh, 0.28)
	default:
		return out
	}
}

func backendLabel(b BackendMode) string {
	switch b {
	case BackendCPU:
		return "CPU"
	case BackendGPU:
		return "OpenCL GPU requested -> CPU fallback"
	default:
		return "Auto -> CPU"
	}
}

func generateGuides(scene *Scene, cam Camera, w, h int) []Guide {
	guides := make([]Guide, w*h)
	aspect := float64(w) / float64(h)
	origin, lower, horiz, vert := cam.basis(aspect)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			u := (float64(x) + 0.5) / float64(iMax(w-1, 1))
			v := (float64(h-1-y) + 0.5) / float64(iMax(h-1, 1))
			d := lower.Add(horiz.Mul(u)).Add(vert.Mul(v)).Sub(origin)
			r := Ray{origin, d}
			if hit, ok := scene.Hit(r, 1e-4, 1e30); ok {
				guides[y*w+x] = Guide{P: r.At(hit.T), N: hit.N, Depth: hit.T, Valid: true}
			}
		}
	}
	return guides
}

func atrousDenoise(img []Vec3, guides []Guide, w, h, iterations int) []Vec3 {
	if len(img) != w*h || len(guides) != w*h || iterations <= 0 {
		return append([]Vec3(nil), img...)
	}
	cur := append([]Vec3(nil), img...)
	next := make([]Vec3, len(img))
	for it := 0; it < iterations; it++ {
		step := 1 << it
		parallelFor(h, runtime.GOMAXPROCS(0), func(ys, ye int) {
			for y := ys; y < ye; y++ {
				for x := 0; x < w; x++ {
					i := y*w + x
					center := cur[i]
					g0 := guides[i]
					l0 := luminance(center)
					var sum Vec3
					wsum := 0.0
					for oy := -1; oy <= 1; oy++ {
						yy := y + oy*step
						if yy < 0 || yy >= h {
							continue
						}
						for ox := -1; ox <= 1; ox++ {
							xx := x + ox*step
							if xx < 0 || xx >= w {
								continue
							}
							j := yy*w + xx
							g1 := guides[j]
							if g0.Valid != g1.Valid {
								continue
							}
							q := cur[j]
							wgt := 1.0
							if g0.Valid {
								nd := clamp(g0.N.Dot(g1.N), -1, 1)
								wgt *= math.Exp((nd - 1) * 48.0)
								depthScale := math.Max(0.15, 0.04*math.Max(g0.Depth, 1.0))
								wgt *= math.Exp(-math.Abs(g0.Depth-g1.Depth) / depthScale)
							}
							wgt *= math.Exp(-math.Abs(l0-luminance(q)) * 3.5)
							if ox == 0 && oy == 0 {
								wgt *= 1.5
							}
							sum = sum.Add(q.Mul(wgt))
							wsum += wgt
						}
					}
					if wsum > 1e-12 {
						next[i] = sum.Mul(1 / wsum)
					} else {
						next[i] = center
					}
				}
			}
		})
		cur, next = next, cur
	}
	return cur
}

func projectPoint(cam Camera, p Vec3, w, h int) (int, int, bool) {
	f := cam.Target.Sub(cam.Position).Unit()
	r := f.Cross(cam.Up).Unit()
	u := r.Cross(f).Unit()
	rel := p.Sub(cam.Position)
	z := rel.Dot(f)
	if z <= 1e-5 {
		return 0, 0, false
	}
	halfH := math.Tan(cam.FOV * math.Pi / 360.0)
	if halfH <= 1e-8 {
		return 0, 0, false
	}
	aspect := float64(w) / float64(h)
	nx := rel.Dot(r) / (z * halfH * aspect)
	ny := rel.Dot(u) / (z * halfH)
	if nx < -1 || nx > 1 || ny < -1 || ny > 1 {
		return 0, 0, false
	}
	x := int(math.Round((nx*0.5 + 0.5) * float64(w-1)))
	y := int(math.Round((0.5 - ny*0.5) * float64(h-1)))
	if x < 0 || x >= w || y < 0 || y >= h {
		return 0, 0, false
	}
	return x, y, true
}

func temporalAccumulate(current []Vec3, guides []Guide, w, h int, prev []Vec3, prevGuides []Guide, prevCam Camera, historyWeight float64) []Vec3 {
	if len(current) != w*h || len(guides) != w*h || len(prev) != w*h || len(prevGuides) != w*h {
		return append([]Vec3(nil), current...)
	}
	historyWeight = clamp(historyWeight, 0, 0.95)
	out := append([]Vec3(nil), current...)
	for i, g := range guides {
		if !g.Valid {
			continue
		}
		x, y, ok := projectPoint(prevCam, g.P, w, h)
		if !ok {
			continue
		}
		j := y*w + x
		pg := prevGuides[j]
		if !pg.Valid {
			continue
		}
		if g.N.Dot(pg.N) < 0.88 {
			continue
		}
		posTol := 0.03 * math.Max(g.Depth, 1.0)
		if g.P.Sub(pg.P).Len() > posTol {
			continue
		}
		// Neighborhood clamp keeps old history from smearing across newly exposed surfaces.
		lo := V(math.Inf(1), math.Inf(1), math.Inf(1))
		hi := V(math.Inf(-1), math.Inf(-1), math.Inf(-1))
		cx := i % w
		cy := i / w
		for oy := -1; oy <= 1; oy++ {
			yy := cy + oy
			if yy < 0 || yy >= h {
				continue
			}
			for ox := -1; ox <= 1; ox++ {
				xx := cx + ox
				if xx < 0 || xx >= w {
					continue
				}
				c := current[yy*w+xx]
				lo = V(math.Min(lo.X, c.X), math.Min(lo.Y, c.Y), math.Min(lo.Z, c.Z))
				hi = V(math.Max(hi.X, c.X), math.Max(hi.Y, c.Y), math.Max(hi.Z, c.Z))
			}
		}
		pc := prev[j]
		pc = V(clamp(pc.X, lo.X, hi.X), clamp(pc.Y, lo.Y, hi.Y), clamp(pc.Z, lo.Z, hi.Z))
		out[i] = current[i].Mul(1 - historyWeight).Add(pc.Mul(historyWeight))
	}
	return out
}

func maxLevel(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func makeTileOrder(tx, ty int, mode string) []int {
	total := tx * ty
	order := make([]int, total)
	for i := 0; i < total; i++ {
		order[i] = i
	}
	if strings.EqualFold(mode, "scanline") || total <= 2 {
		return order
	}
	cx, cy := float64(tx-1)*0.5, float64(ty-1)*0.5
	sort.Slice(order, func(i, j int) bool {
		ai, aj := order[i], order[j]
		ax, ay := float64(ai%tx)-cx, float64(ai/tx)-cy
		bx, by := float64(aj%tx)-cx, float64(aj/tx)-cy
		return ax*ax+ay*ay < bx*bx+by*by
	})
	return order
}

func effectiveSPP(settings *RenderSettings) int {
	msaa := settings.MSAA
	if msaa != 2 && msaa != 4 && msaa != 8 {
		msaa = 1
	}
	tsaa := 1
	switch settings.TSAA {
	case 1:
		tsaa = 2
	case 2:
		tsaa = 3
	case 3:
		tsaa = 4
	}
	v := settings.SPP * msaa * tsaa
	if v < 1 {
		return 1
	}
	if v > 4096 {
		return 4096
	}
	return v
}

func temporalWeight(settings RenderSettings) float64 {
	level := maxLevel(settings.TAA, maxLevel(settings.TSAA, settings.TXAA))
	switch level {
	case 1:
		return 0.48
	case 2:
		return 0.68
	case 3:
		return 0.82
	default:
		return 0
	}
}

func applyHBAO(img []Vec3, guides []Guide, w, h, level int, plus bool) []Vec3 {
	if level <= 0 || len(img) != w*h || len(guides) != w*h {
		return append([]Vec3(nil), img...)
	}
	out := make([]Vec3, len(img))
	radius := 2 + level*2
	if plus {
		radius += 2
	}
	strength := 0.14 + 0.09*float64(level)
	if plus {
		strength *= 1.15
	}
	dirs := [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}}
	if plus {
		dirs = append(dirs, [2]int{2, 1}, [2]int{-2, 1}, [2]int{1, 2}, [2]int{-1, 2})
	}
	parallelFor(h, runtime.GOMAXPROCS(0), func(ys, ye int) {
		for y := ys; y < ye; y++ {
			for x := 0; x < w; x++ {
				i := y*w + x
				g := guides[i]
				if !g.Valid {
					out[i] = img[i]
					continue
				}
				occ, count := 0.0, 0.0
				for _, d := range dirs {
					for step := 1; step <= radius; step += iMax(1, radius/3) {
						xx, yy := x+d[0]*step, y+d[1]*step
						if xx < 0 || xx >= w || yy < 0 || yy >= h {
							continue
						}
						ng := guides[yy*w+xx]
						if !ng.Valid {
							continue
						}
						depthTol := 0.015*math.Max(g.Depth, 1.0) + 0.002*float64(step)
						if ng.Depth+depthTol < g.Depth {
							normalTerm := clamp(1.0-g.N.Dot(ng.N), 0, 1)
							depthTerm := clamp((g.Depth-ng.Depth)/(0.25*math.Max(g.Depth, 1.0)), 0, 1)
							occ += 0.55*depthTerm + 0.45*normalTerm
						}
						count++
					}
				}
				factor := 1.0
				if count > 0 {
					factor = clamp(1.0-strength*(occ/count), 0.35, 1.0)
				}
				out[i] = img[i].Mul(factor)
			}
		}
	})
	return out
}

func boxBlur(img []Vec3, w, h, radius int) []Vec3 {
	if radius <= 0 || len(img) != w*h {
		return append([]Vec3(nil), img...)
	}
	tmp := make([]Vec3, len(img))
	out := make([]Vec3, len(img))
	parallelFor(h, runtime.GOMAXPROCS(0), func(ys, ye int) {
		for y := ys; y < ye; y++ {
			for x := 0; x < w; x++ {
				var sum Vec3
				count := 0.0
				for ox := -radius; ox <= radius; ox++ {
					xx := x + ox
					if xx >= 0 && xx < w {
						sum = sum.Add(img[y*w+xx])
						count++
					}
				}
				tmp[y*w+x] = sum.Mul(1 / math.Max(count, 1))
			}
		}
	})
	parallelFor(w, runtime.GOMAXPROCS(0), func(xs, xe int) {
		for x := xs; x < xe; x++ {
			for y := 0; y < h; y++ {
				var sum Vec3
				count := 0.0
				for oy := -radius; oy <= radius; oy++ {
					yy := y + oy
					if yy >= 0 && yy < h {
						sum = sum.Add(tmp[yy*w+x])
						count++
					}
				}
				out[y*w+x] = sum.Mul(1 / math.Max(count, 1))
			}
		}
	})
	return out
}

func applyBloom(img []Vec3, w, h, level int) []Vec3 {
	if level <= 0 || len(img) != w*h {
		return append([]Vec3(nil), img...)
	}
	bright := make([]Vec3, len(img))
	threshold := 1.25 - 0.2*float64(level)
	parallelFor(len(img), runtime.GOMAXPROCS(0), func(a, b int) {
		for i := a; i < b; i++ {
			c := img[i]
			l := luminance(c)
			if l > threshold {
				bright[i] = c.Mul((l - threshold) / math.Max(l, 1e-6))
			}
		}
	})
	blur := boxBlur(bright, w, h, 1+level)
	strength := 0.08 + 0.09*float64(level)
	out := make([]Vec3, len(img))
	parallelFor(len(img), runtime.GOMAXPROCS(0), func(a, b int) {
		for i := a; i < b; i++ {
			out[i] = img[i].Add(blur[i].Mul(strength))
		}
	})
	return out
}

func applyFXAA(img []Vec3, w, h, level int) []Vec3 {
	if level <= 0 || len(img) != w*h || w < 3 || h < 3 {
		return append([]Vec3(nil), img...)
	}
	out := append([]Vec3(nil), img...)
	threshold := []float64{0, 0.16, 0.10, 0.065}[minInt(level, 3)]
	blend := []float64{0, 0.35, 0.50, 0.65}[minInt(level, 3)]
	parallelFor(h-2, runtime.GOMAXPROCS(0), func(a, b int) {
		for yy := a; yy < b; yy++ {
			y := yy + 1
			for x := 1; x < w-1; x++ {
				i := y*w + x
				c := img[i]
				ln := luminance(img[(y-1)*w+x])
				ls := luminance(img[(y+1)*w+x])
				lw := luminance(img[y*w+x-1])
				le := luminance(img[y*w+x+1])
				lc := luminance(c)
				lo := math.Min(lc, math.Min(math.Min(ln, ls), math.Min(lw, le)))
				hi := math.Max(lc, math.Max(math.Max(ln, ls), math.Max(lw, le)))
				if hi-lo < threshold*math.Max(1.0, hi) {
					continue
				}
				avg := img[(y-1)*w+x].Add(img[(y+1)*w+x]).Add(img[y*w+x-1]).Add(img[y*w+x+1]).Mul(0.25)
				out[i] = c.Mul(1 - blend).Add(avg.Mul(blend))
			}
		}
	})
	return out
}

func applyTXAAResolve(img []Vec3, w, h, level int) []Vec3 {
	if level <= 0 {
		return img
	}
	resolved := applyFXAA(img, w, h, minInt(3, level+1))
	amount := 0.05 + 0.04*float64(level)
	return edgeAwareSharpen(resolved, w, h, amount)
}

func (r *RenderState) Start(scene *Scene, cam Camera, settings RenderSettings) {
	if scene == nil || settings.Width < 16 || settings.Height < 16 || settings.SPP < 1 || settings.Bounces < 1 {
		return
	}
	renderW, renderH := internalSize(settings)
	gen := r.generation.Add(1)
	cancel := &atomic.Bool{}

	r.mu.Lock()
	if r.currentCancel != nil {
		r.currentCancel.Store(true)
	}
	r.currentCancel = cancel
	var prevLinear []Vec3
	var prevGuides []Guide
	prevW, prevH := 0, 0
	prevCam := Camera{}
	prevValid := false
	if temporalWeight(settings) > 0 && r.historyValid {
		prevLinear = append([]Vec3(nil), r.historyLinear...)
		prevGuides = append([]Guide(nil), r.historyGuides...)
		prevW, prevH = r.historyW, r.historyH
		prevCam = r.historyCamera
		prevValid = true
	}
	r.Width, r.Height = renderW, renderH
	r.Pixels = make([]uint32, renderW*renderH)
	r.Linear = make([]Vec3, renderW*renderH)
	r.Guides = nil
	r.Progress = 0
	r.Rendering = true
	r.Status = "Rendering " + settings.Quality + " | " + string(settings.Integrator)
	r.Seconds = 0
	r.Rays = 0
	r.BackendUsed = "Starting"
	r.DeviceName = ""
	r.FrameSerial++
	r.mu.Unlock()

	go func() {
		startTime := time.Now()
		jobLinear := make([]Vec3, renderW*renderH)
		needsGuides := settings.Denoise || settings.HBAO > 0 || settings.HBAOPlus > 0 || temporalWeight(settings) > 0 || settings.DebugView != DebugBeauty
		var jobGuides []Guide
		if needsGuides {
			jobGuides = make([]Guide, renderW*renderH)
		}
		var rays int64
		backendUsed := "CPU"
		deviceName := "Host CPU"
		gpuSucceeded := false
		gpuFailure := ""
		gpuCompatible := settings.Integrator == IntegratorPath && settings.Sampler == SamplerRandom && !settings.MIS && !settings.PowerLightSampling && settings.FireflyClamp <= 0 && !settings.AdaptiveSampling
		allowGPU := settings.Backend != BackendCPU && gpuCompatible
		if settings.Backend == BackendGPU && !gpuCompatible {
			gpuFailure = "specialist CPU transport features enabled; use GPU compatibility preset"
		}

		if allowGPU {
			gpuLinear, gpuGuides, gpuRays, gpuName, err := renderOpenCL(scene, cam, settings, cancel,
				func(y0, rows int, data []Vec3, p float64) {
					if cancel.Load() || r.generation.Load() != gen {
						return
					}
					r.mu.Lock()
					if r.generation.Load() == gen {
						for yy := 0; yy < rows; yy++ {
							dst := (y0 + yy) * renderW
							src := yy * renderW
							copy(r.Linear[dst:dst+renderW], data[src:src+renderW])
							for x := 0; x < renderW; x++ {
								r.Pixels[dst+x] = toRGBA(data[src+x], settings.Exposure, settings.Gamma, settings.HDR)
							}
						}
						r.Progress = p
						r.BackendUsed = "OpenCL GPU"
						r.DeviceName = "OpenCL GPU"
						r.FrameSerial++
					}
					r.mu.Unlock()
				})
			if err == nil && len(gpuLinear) == renderW*renderH && !cancel.Load() {
				jobLinear = gpuLinear
				if needsGuides && len(gpuGuides) == renderW*renderH {
					jobGuides = gpuGuides
				}
				rays = gpuRays
				backendUsed = "OpenCL GPU"
				deviceName = gpuName
				gpuSucceeded = true
			} else if err != nil {
				gpuFailure = err.Error()
			}
		}

		if !gpuSucceeded && !cancel.Load() {
			backendUsed = "CPU"
			if settings.Backend == BackendGPU && gpuFailure != "" {
				backendUsed = "CPU fallback"
				deviceName = gpuFailure
			}
			if settings.Integrator != IntegratorPath {
				deviceName = string(settings.Integrator)
			}
			aspect := float64(renderW) / float64(renderH)
			origin, lower, horiz, vert := cam.basis(aspect)
			tile := settings.TileSize
			if tile <= 0 {
				tile = 16
			}
			tx := (renderW + tile - 1) / tile
			ty := (renderH + tile - 1) / tile
			total := tx * ty
			order := makeTileOrder(tx, ty, settings.TileOrder)
			var jobPixels []uint32
			if settings.ProgressiveUpdates {
				jobPixels = make([]uint32, renderW*renderH)
			}
			var next atomic.Int64
			var done atomic.Int64
			var publishedRays atomic.Int64
			var wg sync.WaitGroup
			workers := minInt(cpuWorkerCount(settings), iMax(1, total))
			targetPublishes := settings.PublishHz
			if targetPublishes <= 0 {
				targetPublishes = 30
			}
			publishEvery := iMax(1, total/targetPublishes)
			// One tile per dynamic claim preserves load balance on noisy path-tracing workloads.
			// The scheduler pays one atomic increment per tile, not per ray/sample.
			chunkSize := 1
			photonMap := PhotonMap{}
			photonRays := int64(0)
			if settings.Integrator == IntegratorPhoton {
				var pc RayCounter
				photonMap = buildPhotonMap(scene, settings, cancel, &pc, time.Now().UnixNano()+int64(gen)*157)
				photonRays = pc.Load()
			}
			sampleSeq := buildSampleSequence(&settings)
			workerRays := make([]int64, workers)
			for wi := 0; wi < workers; wi++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id)*1000003 + int64(gen)*97))
					var localRays RayCounter
					publishedLocal := int64(0)
					defer func() {
						workerRays[id] = localRays.Load()
						if delta := localRays.Load() - publishedLocal; delta > 0 {
							publishedRays.Add(delta)
						}
					}()
					for {
						start := int(next.Add(int64(chunkSize))) - chunkSize
						if start >= total {
							return
						}
						end := minInt(start+chunkSize, total)
						if cancel.Load() || r.generation.Load() != gen {
							return
						}
						for oi := start; oi < end; oi++ {
							ti := order[oi]
							cx, cy := ti%tx, ti/tx
							x0, y0 := cx*tile, cy*tile
							x1, y1 := minInt(x0+tile, renderW), minInt(y0+tile, renderH)
							for y := y0; y < y1; y++ {
								if cancel.Load() || r.generation.Load() != gen {
									return
								}
								row := y * renderW
								for x := x0; x < x1; x++ {
									i := row + x
									var guide *Guide
									if needsGuides {
										guide = &jobGuides[i]
									}
									sum := adaptivePixel(scene, origin, lower, horiz, vert, x, y, renderW, renderH, &settings, rng, &localRays, photonMap, guide, cancel, sampleSeq)
									jobLinear[i] = sum
									if settings.ProgressiveUpdates {
										jobPixels[i] = toRGBA(sum, settings.Exposure, settings.Gamma, settings.HDR)
									}
								}
							}
						}
						if delta := localRays.Load() - publishedLocal; delta > 0 {
							publishedRays.Add(delta)
							publishedLocal += delta
						}
						if cancel.Load() || r.generation.Load() != gen {
							return
						}
						d := done.Add(int64(end - start))
						metadataTick := int(d)%publishEvery < (end-start) || int(d) == total
						if settings.ProgressiveUpdates || metadataTick {
							r.mu.Lock()
							if r.generation.Load() == gen {
								if settings.ProgressiveUpdates {
									for oi := start; oi < end; oi++ {
										ti := order[oi]
										cx, cy := ti%tx, ti/tx
										x0, y0 := cx*tile, cy*tile
										x1, y1 := minInt(x0+tile, renderW), minInt(y0+tile, renderH)
										for y := y0; y < y1; y++ {
											off := y*renderW + x0
											copy(r.Pixels[off:off+(x1-x0)], jobPixels[off:off+(x1-x0)])
											copy(r.Linear[off:off+(x1-x0)], jobLinear[off:off+(x1-x0)])
										}
									}
								}
								r.Progress = float64(d) / float64(total)
								r.Rays = photonRays + publishedRays.Load()
								r.BackendUsed = backendUsed
								r.DeviceName = deviceName
								if settings.ProgressiveUpdates && metadataTick {
									r.FrameSerial++
								}
							}
							r.mu.Unlock()
						}
					}
				}(wi)
			}
			wg.Wait()
			rays = photonRays
			for _, n := range workerRays {
				rays += n
			}
		}

		if r.generation.Load() != gen {
			return
		}
		cancelled := cancel.Load()
		if !cancelled {
			guides := jobGuides
			if gpuSucceeded && needsGuides && len(guides) != renderW*renderH {
				// Safety fallback for an OpenCL driver that cannot return the compact guide buffer.
				guides = generateGuides(scene, cam, renderW, renderH)
			}
			// Geometry-aware AO happens before temporal reconstruction and bloom.
			if settings.HBAOPlus > 0 {
				jobLinear = applyHBAO(jobLinear, guides, renderW, renderH, settings.HBAOPlus, true)
			} else if settings.HBAO > 0 {
				jobLinear = applyHBAO(jobLinear, guides, renderW, renderH, settings.HBAO, false)
			}
			if settings.Denoise {
				passes := 2 + minInt(3, maxLevel(settings.TAA, settings.TXAA))
				jobLinear = atrousDenoise(jobLinear, guides, renderW, renderH, passes)
			}
			tw := temporalWeight(settings)
			if tw > 0 && prevValid && prevW == renderW && prevH == renderH {
				jobLinear = temporalAccumulate(jobLinear, guides, renderW, renderH, prevLinear, prevGuides, prevCam, tw)
			}
			if settings.Bloom > 0 {
				jobLinear = applyBloom(jobLinear, renderW, renderH, settings.Bloom)
			}
			jobLinear = makeDebugView(jobLinear, guides, renderW, renderH, settings)
			// Do not copy full-resolution history unless a temporal mode actually consumes it.
			// This removes a serial memory-bandwidth tail from ordinary path-tracing renders.
			r.mu.Lock()
			if r.generation.Load() == gen {
				if tw > 0 {
					r.historyLinear = append(r.historyLinear[:0], jobLinear...)
					r.historyGuides = append(r.historyGuides[:0], guides...)
					r.historyW = renderW
					r.historyH = renderH
					r.historyCamera = cam
					r.historyValid = true
				} else {
					r.historyValid = false
				}
				if needsGuides {
					r.Guides = append(r.Guides[:0], guides...)
				} else {
					r.Guides = r.Guides[:0]
				}
			}
			r.mu.Unlock()

			finalLinear := jobLinear
			finalW, finalH := renderW, renderH
			if settings.Upscale != UpscaleOff && (renderW != settings.Width || renderH != settings.Height) {
				finalLinear = upscaleDLSSBasic(jobLinear, renderW, renderH, settings.Width, settings.Height, settings.Upscale)
				finalW, finalH = settings.Width, settings.Height
			}
			if settings.FXAA > 0 {
				finalLinear = applyFXAA(finalLinear, finalW, finalH, settings.FXAA)
			}
			if settings.TXAA > 0 {
				finalLinear = applyTXAAResolve(finalLinear, finalW, finalH, settings.TXAA)
			}
			finalPixels := make([]uint32, len(finalLinear))
			parallelFor(len(finalLinear), cpuWorkerCount(settings), func(a, b int) {
				for i := a; i < b; i++ {
					finalPixels[i] = toRGBA(finalLinear[i], settings.Exposure, settings.Gamma, settings.HDR)
				}
			})
			r.mu.Lock()
			if r.generation.Load() == gen {
				r.Width, r.Height = finalW, finalH
				r.Pixels = finalPixels
				r.Linear = finalLinear
				r.Progress = 1
				r.Rays = rays
				r.BackendUsed = backendUsed
				r.DeviceName = deviceName
				r.FrameSerial++
			}
			r.mu.Unlock()
		}

		elapsed := time.Since(startTime).Seconds()
		r.mu.Lock()
		if r.generation.Load() == gen {
			r.Rendering = false
			r.Seconds = elapsed
			r.Rays = rays
			if cancelled {
				r.Status = "Render cancelled"
			} else {
				r.Status = "Render complete"
			}
			if r.currentCancel == cancel {
				r.currentCancel = nil
			}
			r.FrameSerial++
		}
		r.mu.Unlock()
	}()
}

func iMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func NewShowcaseScene() (*Scene, Camera) {
	s := &Scene{Name: "Material Showcase"}
	ground := s.AddMaterial(Material{Kind: MatDiffuse, Color: V(.55, .57, .6)})
	red := s.AddMaterial(Material{Kind: MatDiffuse, Color: V(.72, .12, .10)})
	gold := s.AddMaterial(Material{Kind: MatMetal, Color: V(.93, .72, .28), Roughness: .08})
	glass := s.AddMaterial(Material{Kind: MatGlass, Color: V(.98, .99, 1), IOR: 1.5})
	light := s.AddMaterial(Material{Kind: MatEmissive, Emission: V(18, 15, 11)})
	s.AddSphere(V(0, -1001, -4), 1000, ground)
	s.AddSphere(V(-1.8, 0, -4.2), 1, red)
	s.AddSphere(V(.25, 0, -3.2), 1, glass)
	s.AddSphere(V(2.2, 0, -4.4), 1, gold)
	s.AddSphere(V(0, 5, -4), .8, light)
	s.Build()
	c := DefaultCamera()
	return s, c
}
func NewCornellScene() (*Scene, Camera) {
	s := &Scene{Name: "Cornell-like Room"}
	white := s.AddMaterial(Material{Kind: MatDiffuse, Color: V(.72, .72, .72)})
	red := s.AddMaterial(Material{Kind: MatDiffuse, Color: V(.70, .08, .06)})
	green := s.AddMaterial(Material{Kind: MatDiffuse, Color: V(.08, .55, .12)})
	glass := s.AddMaterial(Material{Kind: MatGlass, Color: V(.98, .99, 1), IOR: 1.5})
	light := s.AddMaterial(Material{Kind: MatEmissive, Emission: V(22, 20, 16)})
	s.AddTriangle(V(-3, -2, -8), V(3, -2, -8), V(3, -2, -2), white)
	s.AddTriangle(V(-3, -2, -8), V(3, -2, -2), V(-3, -2, -2), white)
	s.AddTriangle(V(-3, -2, -8), V(-3, -2, -2), V(-3, 4, -2), red)
	s.AddTriangle(V(-3, -2, -8), V(-3, 4, -2), V(-3, 4, -8), red)
	s.AddTriangle(V(3, -2, -2), V(3, -2, -8), V(3, 4, -8), green)
	s.AddTriangle(V(3, -2, -2), V(3, 4, -8), V(3, 4, -2), green)
	s.AddSphere(V(-.9, -1, -5.2), 1, glass)
	s.AddSphere(V(1.1, -1.2, -4.2), .8, white)
	s.AddSphere(V(0, 3.2, -5), .65, light)
	s.Build()
	c := Camera{V(0, .5, 5), V(0, 0, -5), V(0, 1, 0), 38}
	return s, c
}
func NewStressScene() (*Scene, Camera) {
	s := &Scene{Name: "Stress Grid"}
	ground := s.AddMaterial(Material{Kind: MatDiffuse, Color: V(.45, .48, .52)})
	blue := s.AddMaterial(Material{Kind: MatMetal, Color: V(.25, .52, .92), Roughness: .12})
	light := s.AddMaterial(Material{Kind: MatEmissive, Emission: V(25, 22, 18)})
	s.AddSphere(V(0, -1001, -10), 1000, ground)
	for z := 0; z < 20; z++ {
		for x := -20; x <= 20; x++ {
			s.AddSphere(V(float64(x)*.58, -.65, -3-float64(z)*.58), .22, blue)
		}
	}
	s.AddSphere(V(0, 8, -8), 1.2, light)
	s.Build()
	return s, Camera{V(9, 5, 8), V(0, -.5, -8), V(0, 1, 0), 42}
}

func LoadOBJ(path string) (*Scene, Camera, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, Camera{}, err
	}
	defer f.Close()
	s := &Scene{Name: path}
	mat := s.AddMaterial(Material{Kind: MatDiffuse, Color: V(.72, .76, .82)})
	verts := []Vec3{}
	mn := V(math.Inf(1), math.Inf(1), math.Inf(1))
	mx := V(math.Inf(-1), math.Inf(-1), math.Inf(-1))
	scan := bufio.NewScanner(f)
	lineNo := 0
	for scan.Scan() {
		lineNo++
		line := strings.TrimSpace(scan.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		if parts[0] == "v" {
			if len(parts) < 4 {
				return nil, Camera{}, fmt.Errorf("OBJ line %d: malformed vertex", lineNo)
			}
			x, e1 := strconv.ParseFloat(parts[1], 64)
			y, e2 := strconv.ParseFloat(parts[2], 64)
			z, e3 := strconv.ParseFloat(parts[3], 64)
			if e1 != nil || e2 != nil || e3 != nil {
				return nil, Camera{}, fmt.Errorf("OBJ line %d: invalid vertex", lineNo)
			}
			v := V(x, y, z)
			verts = append(verts, v)
			mn = V(math.Min(mn.X, x), math.Min(mn.Y, y), math.Min(mn.Z, z))
			mx = V(math.Max(mx.X, x), math.Max(mx.Y, y), math.Max(mx.Z, z))
		} else if parts[0] == "f" {
			if len(parts) < 4 {
				return nil, Camera{}, fmt.Errorf("OBJ line %d: face needs 3 vertices", lineNo)
			}
			idx := make([]int, 0, len(parts)-1)
			for _, tok := range parts[1:] {
				head := strings.Split(tok, "/")[0]
				n, e := strconv.Atoi(head)
				if e != nil || n == 0 {
					return nil, Camera{}, fmt.Errorf("OBJ line %d: invalid index", lineNo)
				}
				if n < 0 {
					n = len(verts) + n + 1
				}
				n--
				if n < 0 || n >= len(verts) {
					return nil, Camera{}, fmt.Errorf("OBJ line %d: index out of range", lineNo)
				}
				idx = append(idx, n)
			}
			for i := 1; i+1 < len(idx); i++ {
				s.AddTriangle(verts[idx[0]], verts[idx[i]], verts[idx[i+1]], mat)
			}
		}
	}
	if err := scan.Err(); err != nil {
		return nil, Camera{}, err
	}
	if len(s.Primitives) == 0 {
		return nil, Camera{}, errors.New("OBJ contains no renderable faces")
	}
	s.Build()
	center := mn.Add(mx).Mul(.5)
	rad := math.Max(.5, mx.Sub(mn).Len()*.5)
	cam := Camera{center.Add(V(rad*1.4, rad*.9, rad*2.6)), center, V(0, 1, 0), 42}
	return s, cam, nil
}

func RotatedDimensions(w, h, rot int) (int, int) {
	rot = ((rot % 4) + 4) % 4
	if rot%2 == 1 {
		return h, w
	}
	return w, h
}
func RotatePixels(p []uint32, w, h, rot int) ([]uint32, int, int) {
	rot = ((rot % 4) + 4) % 4
	dw, dh := RotatedDimensions(w, h, rot)
	out := make([]uint32, dw*dh)
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			sx, sy := x, y
			switch rot {
			case 1:
				sx = y
				sy = h - 1 - x
			case 2:
				sx = w - 1 - x
				sy = h - 1 - y
			case 3:
				sx = w - 1 - y
				sy = x
			}
			if sx >= 0 && sx < w && sy >= 0 && sy < h {
				out[y*dw+x] = p[sy*w+sx]
			}
		}
	}
	return out, dw, dh
}

func SavePPM(path string, pix []uint32, w, h, rot int) error {
	p, rw, rh := RotatePixels(pix, w, h, rot)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	bw := bufio.NewWriterSize(f, 1<<20)
	fmt.Fprintf(bw, "P6\n%d %d\n255\n", rw, rh)
	buf := make([]byte, 3*rw)
	for y := 0; y < rh; y++ {
		for x := 0; x < rw; x++ {
			v := p[y*rw+x]
			buf[x*3] = byte(v)
			buf[x*3+1] = byte(v >> 8)
			buf[x*3+2] = byte(v >> 16)
		}
		if _, err = bw.Write(buf); err != nil {
			return err
		}
	}
	return bw.Flush()
}

type RenderMeta struct {
	Width, Height int
	Progress      float64
	Rendering     bool
	Status        string
	Seconds       float64
	Rays          int64
	FrameSerial   uint64
	BackendUsed   string
	DeviceName    string
}

func (r *RenderState) Meta() RenderMeta {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return RenderMeta{r.Width, r.Height, r.Progress, r.Rendering, r.Status, r.Seconds, r.Rays, r.FrameSerial, r.BackendUsed, r.DeviceName}
}
func (r *RenderState) PixelsCopy() (int, int, []uint32) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Width, r.Height, append([]uint32(nil), r.Pixels...)
}

func SaveBMP(path string, pix []uint32, w, h, rot int) error {
	p, rw, rh := RotatePixels(pix, w, h, rot)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	rowBytes := rw * 4
	dataSize := rowBytes * rh
	fileSize := 14 + 40 + dataSize
	if _, err = f.Write([]byte{'B', 'M'}); err != nil {
		return err
	}
	vals := []any{uint32(fileSize), uint16(0), uint16(0), uint32(54), uint32(40), int32(rw), int32(-rh), uint16(1), uint16(32), uint32(0), uint32(dataSize), int32(2835), int32(2835), uint32(0), uint32(0)}
	for _, v := range vals {
		if err = binary.Write(f, binary.LittleEndian, v); err != nil {
			return err
		}
	}
	buf := make([]byte, rowBytes)
	for y := 0; y < rh; y++ {
		for x := 0; x < rw; x++ {
			v := p[y*rw+x]
			i := x * 4
			buf[i] = byte(v >> 16)
			buf[i+1] = byte(v >> 8)
			buf[i+2] = byte(v)
			buf[i+3] = 255
		}
		if _, err = f.Write(buf); err != nil {
			return err
		}
	}
	return nil
}

func RotateLinear(p []Vec3, w, h, rot int) ([]Vec3, int, int) {
	rot = ((rot % 4) + 4) % 4
	dw, dh := RotatedDimensions(w, h, rot)
	out := make([]Vec3, dw*dh)
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			sx, sy := x, y
			switch rot {
			case 1:
				sx = y
				sy = h - 1 - x
			case 2:
				sx = w - 1 - x
				sy = h - 1 - y
			case 3:
				sx = w - 1 - y
				sy = x
			}
			if sx >= 0 && sx < w && sy >= 0 && sy < h {
				out[y*dw+x] = p[sy*w+sx]
			}
		}
	}
	return out, dw, dh
}

func SavePFM(path string, linear []Vec3, w, h, rot int) error {
	p, rw, rh := RotateLinear(linear, w, h, rot)
	if len(p) != rw*rh || rw <= 0 || rh <= 0 {
		return errors.New("invalid HDR image")
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	bw := bufio.NewWriterSize(f, 1<<20)
	if _, err = fmt.Fprintf(bw, "PF\n%d %d\n-1.0\n", rw, rh); err != nil {
		return err
	}
	// PFM rows are conventionally written bottom-to-top.
	for y := rh - 1; y >= 0; y-- {
		for x := 0; x < rw; x++ {
			c := p[y*rw+x]
			for _, v := range []float32{float32(c.X), float32(c.Y), float32(c.Z)} {
				if err = binary.Write(bw, binary.LittleEndian, v); err != nil {
					return err
				}
			}
		}
	}
	return bw.Flush()
}

func (r *RenderState) LinearCopy() (int, int, []Vec3) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Width, r.Height, append([]Vec3(nil), r.Linear...)
}

// threadedBVHEscapes returns a stackless depth-first escape index for each BVH node.
// On a node miss or after finishing a leaf, traversal jumps directly to escape[node].
// Internal nodes descend through Left; the left subtree escapes to Right and the right
// subtree escapes to the parent's escape. This removes the per-ray traversal stack on GPU.
func threadedBVHEscapes(nodes []BVHNode) []int32 {
	escape := make([]int32, len(nodes))
	for i := range escape {
		escape[i] = -1
	}
	if len(nodes) == 0 {
		return escape
	}
	var assign func(int, int32)
	assign = func(idx int, next int32) {
		if idx < 0 || idx >= len(nodes) {
			return
		}
		escape[idx] = next
		n := nodes[idx]
		if n.Count > 0 {
			return
		}
		if n.Left >= 0 && n.Right >= 0 {
			assign(n.Left, int32(n.Right))
			assign(n.Right, next)
		}
	}
	assign(0, -1)
	return escape
}
