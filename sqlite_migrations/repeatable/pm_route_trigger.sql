DROP TRIGGER IF EXISTS pm_route_after_insert_trigger;
CREATE TRIGGER pm_route_after_insert_trigger AFTER INSERT ON pm_route
FOR EACH ROW WHEN NEW.parent_route_id IS NOT NULL
BEGIN
    INSERT INTO pm_route_closure (route_id, ancestor_route_id, depth)
    SELECT NEW.route_id, ancestor_route_id, depth
    FROM pm_route_closure
    WHERE route_id = NEW.parent_route_id
    UNION ALL
    SELECT NEW.route_id, NEW.parent_route_id, COALESCE(MAX(depth), 0)+1
    FROM pm_route_closure
    WHERE route_id = NEW.parent_route_id;

    UPDATE pm_route
    SET path = (
        SELECT group_concat(ancestors.basename, '/') || '/' || pm_route.basename AS path
        FROM (
            SELECT ancestor.basename
            FROM pm_route_closure JOIN pm_route AS ancestor ON ancestor.route_id = pm_route_closure.ancestor_route_id
            WHERE pm_route_closure.route_id = pm_route.route_id
            ORDER BY pm_route_closure.depth
        ) AS ancestors
    )
    WHERE pm_route.route_id = NEW.route_id;
END;

DROP TRIGGER IF EXISTS pm_route_after_delete_trigger;
CREATE TRIGGER pm_route_after_delete_trigger AFTER DELETE ON pm_route
BEGIN
    DELETE FROM pm_route_closure WHERE route_id = OLD.route_id;
END;

DROP TRIGGER IF EXISTS pm_route_path_after_update_trigger;
CREATE TRIGGER pm_route_path_after_update_trigger AFTER UPDATE ON pm_route
FOR EACH ROW WHEN OLD.basename <> NEW.basename
BEGIN
    UPDATE pm_route
    SET path = (
        SELECT group_concat(ancestors.basename, '/') || '/' || pm_route.basename AS path
        FROM (
            SELECT ancestor.basename
            FROM pm_route_closure JOIN pm_route AS ancestor ON ancestor.route_id = pm_route_closure.ancestor_route_id
            WHERE pm_route_closure.route_id = pm_route.route_id
            ORDER BY pm_route_closure.depth
        ) AS ancestors
    )
    WHERE pm_route.route_id = NEW.route_id OR EXISTS (
        SELECT 1 FROM pm_route_closure WHERE ancestor_route_id = NEW.route_id
    );
END;
