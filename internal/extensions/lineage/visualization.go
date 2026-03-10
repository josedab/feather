package lineage

import "fmt"

// GenerateVisualizationHTML generates an HTML page with D3.js-powered DAG visualization.
func (u *UnifiedLineage) GenerateVisualizationHTML() string {
	graph := u.GetGraph()

	// Convert graph data to JavaScript-embeddable JSON
	nodesJS := "["
	for i, node := range graph.Nodes {
		if i > 0 {
			nodesJS += ","
		}
		color := "#e0e0e0"
		switch node.Kind {
		case UnifiedNodeSource:
			color = "#4fc3f7"
		case UnifiedNodeFeature:
			color = "#fff176"
		case UnifiedNodeModel:
			color = "#81c784"
		case UnifiedNodeConsumer:
			color = "#ef9a9a"
		case UnifiedNodeTransform:
			color = "#ce93d8"
		}
		if node.SLAViolation {
			color = "#f44336"
		}
		nodesJS += fmt.Sprintf(`{"id":%q,"name":%q,"kind":%q,"color":%q,"quality":%.2f,"drift":%.2f,"freshness":%d,"slaViolation":%t}`,
			node.ID, node.Name, node.Kind, color, node.QualityScore, node.DriftScore, node.FreshnessMs, node.SLAViolation)
	}
	nodesJS += "]"

	linksJS := "["
	for i, edge := range graph.Edges {
		if i > 0 {
			linksJS += ","
		}
		linksJS += fmt.Sprintf(`{"source":%q,"target":%q,"label":%q}`, edge.From, edge.To, edge.Label)
	}
	linksJS += "]"

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<title>Feather Lineage Explorer</title>
<script src="https://d3js.org/d3.v7.min.js"></script>
<style>
  body { margin: 0; font-family: system-ui, sans-serif; background: #1a1a2e; color: #e0e0e0; }
  #controls { padding: 12px; background: #16213e; display: flex; gap: 12px; align-items: center; }
  #controls button { padding: 6px 14px; border: 1px solid #444; border-radius: 4px; background: #0f3460; color: #e0e0e0; cursor: pointer; }
  #controls button:hover { background: #533483; }
  #controls input { padding: 6px; border: 1px solid #444; border-radius: 4px; background: #0f3460; color: #e0e0e0; width: 200px; }
  #graph { width: 100vw; height: calc(100vh - 48px); }
  .node rect { stroke: #333; stroke-width: 1.5px; rx: 6; }
  .node text { font-size: 11px; fill: #333; }
  .link { stroke: #666; stroke-width: 1.5px; fill: none; marker-end: url(#arrow); }
  .link-label { font-size: 9px; fill: #aaa; }
  #tooltip { position: absolute; padding: 8px 12px; background: #16213e; border: 1px solid #444; border-radius: 6px; font-size: 12px; pointer-events: none; display: none; }
</style>
</head>
<body>
<div id="controls">
  <strong>Feather Lineage Explorer</strong>
  <input type="text" id="search" placeholder="Filter nodes...">
  <button onclick="resetView()">Reset View</button>
  <button onclick="highlightSLA()">Show SLA Violations</button>
  <span id="stats"></span>
</div>
<div id="graph"></div>
<div id="tooltip"></div>
<script>
const nodes = %s;
const links = %s;
document.getElementById('stats').textContent = nodes.length + ' nodes, ' + links.length + ' edges';

const width = window.innerWidth;
const height = window.innerHeight - 48;
const svg = d3.select('#graph').append('svg').attr('width', width).attr('height', height);
const g = svg.append('g');

svg.append('defs').append('marker').attr('id','arrow').attr('viewBox','0 -5 10 10')
  .attr('refX',20).attr('refY',0).attr('markerWidth',6).attr('markerHeight',6)
  .attr('orient','auto').append('path').attr('d','M0,-5L10,0L0,5').attr('fill','#666');

const zoom = d3.zoom().scaleExtent([0.1,4]).on('zoom', e => g.attr('transform', e.transform));
svg.call(zoom);

const sim = d3.forceSimulation(nodes)
  .force('link', d3.forceLink(links).id(d=>d.id).distance(120))
  .force('charge', d3.forceManyBody().strength(-300))
  .force('center', d3.forceCenter(width/2, height/2))
  .force('y', d3.forceY(height/2).strength(0.05));

const link = g.selectAll('.link').data(links).enter().append('line').attr('class','link');
const linkLabel = g.selectAll('.link-label').data(links).enter().append('text').attr('class','link-label')
  .text(d=>d.label).attr('text-anchor','middle');

const node = g.selectAll('.node').data(nodes).enter().append('g').attr('class','node')
  .call(d3.drag().on('start',ds).on('drag',dd).on('end',de));
node.append('rect').attr('width',100).attr('height',36).attr('x',-50).attr('y',-18)
  .attr('fill',d=>d.color).attr('opacity',0.9);
node.append('text').attr('text-anchor','middle').attr('dy','0.35em').text(d=>d.name);

const tooltip = d3.select('#tooltip');
node.on('mouseover',(e,d)=>{
  tooltip.style('display','block').style('left',(e.pageX+10)+'px').style('top',(e.pageY-10)+'px')
    .html('<b>'+d.name+'</b><br>Kind: '+d.kind+'<br>Quality: '+d.quality.toFixed(2)+'<br>Drift: '+d.drift.toFixed(2)+'<br>Freshness: '+d.freshness+'ms'+(d.slaViolation?'<br><span style="color:red">SLA VIOLATION</span>':''));
}).on('mouseout',()=>tooltip.style('display','none'));

sim.on('tick',()=>{
  link.attr('x1',d=>d.source.x).attr('y1',d=>d.source.y).attr('x2',d=>d.target.x).attr('y2',d=>d.target.y);
  linkLabel.attr('x',d=>(d.source.x+d.target.x)/2).attr('y',d=>(d.source.y+d.target.y)/2);
  node.attr('transform',d=>'translate('+d.x+','+d.y+')');
});

function ds(e,d){if(!e.active)sim.alphaTarget(0.3).restart();d.fx=d.x;d.fy=d.y;}
function dd(e,d){d.fx=e.x;d.fy=e.y;}
function de(e,d){if(!e.active)sim.alphaTarget(0);d.fx=null;d.fy=null;}

function resetView(){svg.transition().call(zoom.transform,d3.zoomIdentity);}
function highlightSLA(){node.select('rect').attr('stroke',d=>d.slaViolation?'red':'#333').attr('stroke-width',d=>d.slaViolation?3:1.5);}

document.getElementById('search').addEventListener('input', e => {
  const q = e.target.value.toLowerCase();
  node.attr('opacity', d => (!q || d.name.toLowerCase().includes(q) || d.kind.includes(q)) ? 1 : 0.15);
  link.attr('opacity', d => (!q || d.source.name.toLowerCase().includes(q) || d.target.name.toLowerCase().includes(q)) ? 1 : 0.05);
});
</script>
</body>
</html>`, nodesJS, linksJS)
}
