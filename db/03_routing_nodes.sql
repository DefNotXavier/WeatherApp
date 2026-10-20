-- Routing nodes: every vertex of every highway line, in order.
--
-- The Go server loads these to build its routing graph, and snaps clicked
-- points to the nearest one with a KNN (<->) query on the GIST index. Junctions
-- like Butte and Billings are a vertex on both lines with the same coordinates,
-- which is what connects the highways in the graph.
--
-- It's a materialized view because it only changes when the highways do:
--   REFRESH MATERIALIZED VIEW highway_nodes;
CREATE MATERIALIZED VIEW highway_nodes AS
SELECT h.name       AS highway,
       (d).path[1]  AS seq,
       (d).geom     AS geom
FROM highways h,
     LATERAL ST_DumpPoints(h.geom) AS d;

CREATE INDEX highway_nodes_geom_idx ON highway_nodes USING GIST (geom);
