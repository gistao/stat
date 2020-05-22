package stat

import (
	"container/list"
	"encoding/json"
	"fmt"
	"github.com/wcharczuk/go-chart"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type StatType uint8

const (
	STQPS     StatType = 0x01
	STQPSPeak StatType = 0x02 // STQPSPeak contains STQPS
	STSum     StatType = 0x04
	STVal     StatType = 0x08
	stMin     StatType = 0x01
	stMax     StatType = 0x10

	DefaultBufSize int = 20000
)

var (
	stSuffix = map[StatType]string{
		STQPS:     "_qps",
		STQPSPeak: "_qpk",
		STSum:     "_sum",
		STVal:     "_val",
	}
	stTypeName = map[StatType]string{
		STQPS:     "qps",
		STQPSPeak: "qps_peak",
		STSum:     "sum",
		STVal:     "val",
	}
)

// SIG must get it from NewSIG
type SIG struct {
	sig chan interface{}
	clo chan struct{}
}

// NewSIG make a SIG
func NewSIG(len int) *SIG {
	return &SIG{
		sig: make(chan interface{}, len),
		clo: make(chan struct{}),
	}
}

// Signal something happen
func (s *SIG) Signal(v interface{}, timeout time.Duration) {
	select {
	case <-time.After(timeout):
		return
	case s.sig <- v:
	}
}

// Wait something happen
func (s *SIG) Wait() <-chan interface{} {
	return s.sig
}

// Close a wait goroutine
func (s *SIG) Close() {
	close(s.clo)
}

type statKV struct {
	group string
	key   string
	val   int64
}

type StatKey struct {
	Group  string
	Name   string
	STName map[StatType]string
	ST     StatType
}

func (p *StatKey) GetName(st StatType) string {
	if v, ok := p.STName[st]; ok {
		return v
	}
	return p.Name + stSuffix[st]
}

type kvIn map[string]map[string]int64
type kvOt map[string]map[StatType]interface{}
type kvWt map[string]map[string]*list.List
type Handler func(w http.ResponseWriter, req *http.Request)

type Stat struct {
	groups       []string
	keys         map[string]map[string]*StatKey
	stKeys       map[string]map[StatType][]string
	stData       kvOt
	stDataLock   *sync.Mutex
	result       map[string]string
	resultLock   *sync.Mutex
	baseData     kvIn
	baseLock     *sync.Mutex
	baseDataBak  kvIn
	baseDataLast kvIn
	watchData    kvWt
	isWatch      bool
	timeLast     int64
	dataCh       chan *statKV
	sig          *SIG
	interval     time.Duration
	closeCh      chan struct{}
}

func (p *Stat) Init(interval time.Duration, bufsize int, h *Handler, sig *SIG) {
	p.groups = make([]string, 0)
	p.keys = make(map[string]map[string]*StatKey)
	p.stKeys = make(map[string]map[StatType][]string)
	p.stData = make(kvOt)
	p.stDataLock = new(sync.Mutex)
	p.result = make(map[string]string)
	p.resultLock = new(sync.Mutex)
	p.baseData = make(kvIn)
	p.baseDataBak = make(kvIn)
	p.baseDataLast = make(kvIn)
	p.baseLock = new(sync.Mutex)
	p.watchData = make(kvWt)
	p.timeLast = time.Now().UnixNano()
	if bufsize < DefaultBufSize {
		bufsize = DefaultBufSize
	}
	p.dataCh = make(chan *statKV, bufsize)
	p.closeCh = make(chan struct{})
	p.sig = sig
	p.interval = interval
	if h != nil {
		*h = p.watch
		p.isWatch = true
	}

	go p.handleData()
	go p.doStat()
}

func (p *Stat) Stop() {
	close(p.closeCh)
}

func (p *Stat) RegisterKey(key *StatKey) {
	p.stDataLock.Lock()
	defer p.stDataLock.Unlock()

	if key.ST&STQPSPeak == STQPSPeak {
		key.ST |= STQPS
	}
	if key.ST&STVal == STVal {
		key.ST = STVal
	}

	if _, ok := p.keys[key.Group]; !ok {
		p.groups = append(p.groups, key.Group)
		sort.Strings(p.groups)
		p.keys[key.Group] = make(map[string]*StatKey)
		p.stKeys[key.Group] = make(map[StatType][]string)
		p.stData[key.Group] = make(map[StatType]interface{})
		p.baseData[key.Group] = make(map[string]int64)
		p.baseDataBak[key.Group] = make(map[string]int64)
		p.baseDataLast[key.Group] = make(map[string]int64)
		p.watchData[key.Group] = make(map[string]*list.List)
	}

	for t := stMin; t < stMax; t = t << 1 {
		if key.ST&t == t {
			if _, ok := p.stKeys[key.Group][t]; !ok {
				p.stKeys[key.Group][t] = make([]string, 0)
			}
			idx := sort.SearchStrings(p.stKeys[key.Group][t], key.Name)
			if idx == len(p.stKeys[key.Group][t]) || p.stKeys[key.Group][t][idx] != key.Name {
				p.stKeys[key.Group][t] = append(p.stKeys[key.Group][t], key.Name)
				sort.Strings(p.stKeys[key.Group][t])
				p.keys[key.Group][key.Name] = key
			}

			if _, ok := p.watchData[key.Group][key.GetName(t)]; !ok {
				p.watchData[key.Group][key.GetName(t)] = list.New()
			}

			// init stData
			switch t {
			case STQPS:
				if _, ok := p.stData[key.Group][t]; !ok {
					p.stData[key.Group][t] = make(map[string]float64)
				}
			case STQPSPeak:
				if _, ok := p.stData[key.Group][t]; !ok {
					p.stData[key.Group][t] = make(map[string]float64)
				}
			case STSum:
				if _, ok := p.stData[key.Group][t]; !ok {
					p.stData[key.Group][t] = make(map[string]int64)
				}
			case STVal:
				if _, ok := p.stData[key.Group][t]; !ok {
					p.stData[key.Group][t] = make(map[string]int64)
				}
			}
		}
	}
}

func (p *Stat) AddVal(group, key string, val int64) {
	p.dataCh <- &statKV{group, key, val}
}

func (p *Stat) GetJsonInfo() ([]byte, error) {
	tmp := make(map[string]map[string]interface{})
	p.stDataLock.Lock()
	for g, gd := range p.stData {
		tmp[g] = make(map[string]interface{})
		for st, sd := range gd {
			kd := make(map[string]int64)
			if st&STQPS == STQPS || st&STQPSPeak == STQPSPeak {
				for k, v := range sd.(map[string]float64) {
					kd[p.keys[g][k].GetName(st)] = int64(v)
				}
			}
			if st&STSum == STSum || st&STVal == STVal {
				for k, v := range sd.(map[string]int64) {
					kd[p.keys[g][k].GetName(st)] = v
				}
			}
			tmp[g][stTypeName[st]] = kd
		}
	}
	p.stDataLock.Unlock()

	return json.Marshal(tmp)
}

func (p *Stat) GetStringsInfo() []string {
	info := make([]string, 0)
	p.resultLock.Lock()
	for _, g := range p.groups {
		info = append(info, p.result[g])
	}
	p.resultLock.Unlock()

	return info
}

func (p *Stat) doStat() {
	d := p.interval
	for {
		select {
		case <-p.closeCh:
			return

		case <-time.After(d):
			timeNow := time.Now().UnixNano()
			p.baseLock.Lock()
			for g, gd := range p.baseData {
				for k, v := range gd {
					p.baseDataBak[g][k] = v
				}
			}
			p.baseLock.Unlock()
			timeDelta := float64(timeNow-p.timeLast) / 1000000000
			if timeDelta == 0 {
				continue
			}
			p.timeLast = timeNow

			p.stDataLock.Lock()
			for group, stKeys := range p.stKeys {
				var line string
				for t := stMin; t < stMax; t = t << 1 {
					if _, ok := stKeys[t]; !ok {
						continue
					}

					switch t {
					case STQPS:
						data := p.stData[group][t].(map[string]float64)
						for _, k := range p.stKeys[group][t] {
							data[k] = float64(p.baseDataBak[group][k]-p.baseDataLast[group][k]) / timeDelta
							line += fmt.Sprintf("%s=%.0f/s, ", p.keys[group][k].GetName(t), data[k])
							if p.isWatch {
								l := p.watchData[group][p.keys[group][k].GetName(t)]
								l.PushBack(data[k])
								if l.Len() > 600 {
									l.Remove(l.Front())
								}
							}
						}
					case STQPSPeak:
						data := p.stData[group][t].(map[string]float64)
						dataQPS := p.stData[group][STQPS].(map[string]float64)
						for _, k := range p.stKeys[group][t] {
							if data[k] < dataQPS[k] {
								data[k] = dataQPS[k]
							}
							line += fmt.Sprintf("%s=%.0f/s, ", p.keys[group][k].GetName(t), data[k])
							if p.isWatch {
								l := p.watchData[group][p.keys[group][k].GetName(t)]
								l.PushBack(data[k])
								if l.Len() > 600 {
									l.Remove(l.Front())
								}
							}
						}
					case STSum:
						data := p.stData[group][t].(map[string]int64)
						for _, k := range p.stKeys[group][t] {
							data[k] = p.baseDataBak[group][k]
							line += fmt.Sprintf("%s=%d, ", p.keys[group][k].GetName(t), data[k])
							if p.isWatch {
								l := p.watchData[group][p.keys[group][k].GetName(t)]
								l.PushBack(data[k])
								if l.Len() > 600 {
									l.Remove(l.Front())
								}
							}
						}
					case STVal:
						data := p.stData[group][t].(map[string]int64)
						for _, k := range p.stKeys[group][t] {
							data[k] = p.baseDataBak[group][k]
							line += fmt.Sprintf("%s=%d, ", p.keys[group][k].GetName(t), data[k])
							if p.isWatch {
								l := p.watchData[group][p.keys[group][k].GetName(t)]
								l.PushBack(data[k])
								if l.Len() > 600 {
									l.Remove(l.Front())
								}
							}
						}
					}
				}
				if line != "" {
					line = strings.TrimSuffix(line, ", ")
					p.resultLock.Lock()
					p.result[group] = group + ": " + line
					p.resultLock.Unlock()
				}
			}
			p.stDataLock.Unlock()

			if p.sig != nil {
				p.sig.Signal(struct{}{}, time.Nanosecond)
			}
			for g, gd := range p.baseDataBak {
				for k, v := range gd {
					p.baseDataLast[g][k] = v
				}
			}
			d = p.interval - time.Duration(time.Now().UnixNano()-timeNow)
		}
	}
}

func (p *Stat) handleData() {
	for {
		select {
		case <-p.closeCh:
			return

		case kv := <-p.dataCh:
			p.baseLock.Lock()
			for i, len := 0, len(p.dataCh); ; {
				if p.keys[kv.group][kv.key].ST&STVal == STVal {
					p.baseData[kv.group][kv.key] = kv.val
				} else {
					p.baseData[kv.group][kv.key] += kv.val
				}

				if i++; i > len {
					break
				}
				kv = <-p.dataCh
			}
			p.baseLock.Unlock()
		}
	}
}

func (p *Stat) watch(w http.ResponseWriter, req *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			w.WriteHeader(400)
			w.Write([]byte(fmt.Sprint(err)))
		}
	}()

	err := req.ParseForm()
	if err != nil {
		return
	}

	graph := chart.Chart{
		XAxis: chart.XAxis{
			Style: chart.StyleShow(),
		},
		YAxis: chart.YAxis{
			Style: chart.StyleShow(),
		},
		Background: chart.Style{
			Padding: chart.Box{
				Top:  20,
				Left: 20,
			},
		},
	}

	p.stDataLock.Lock()
	for g, v := range req.Form {
		for _, e := range v {
			if _, ok := p.watchData[g]; !ok {
				continue
			}
			if _, ok := p.watchData[g][e]; !ok {
				continue
			}
			l := p.watchData[g][e]
			c := chart.ContinuousSeries{
				Name: g + "." + e,
			}

			x := float64(0.0)
			for m := l.Front(); m != nil; m = m.Next() {
				y, ok := m.Value.(float64)
				if !ok {
					y = float64(m.Value.(int64))
				}
				x += 1.0
				c.XValues = append(c.XValues, x)
				c.YValues = append(c.YValues, y)
			}
			graph.Series = append(graph.Series, c)
		}
	}
	p.stDataLock.Unlock()
	//note we have to do this as a separate step because we need a reference to graph
	graph.Elements = []chart.Renderable{
		chart.Legend(&graph),
	}

	w.Header().Set("Content-Type", "image/png")
	graph.Render(chart.PNG, w)
}
