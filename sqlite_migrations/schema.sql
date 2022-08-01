CREATE TABLE pm_site (
    site_id UUID PRIMARY KEY NOT NULL
    ,domain TEXT
    ,subdomain TEXT
    ,tilde_prefix TEXT
    ,data JSON
    ,is_primary BOOLEAN

    ,CONSTRAINT pm_site_domain_subdomain_tilde_prefix_key UNIQUE (domain, subdomain, tilde_prefix)
    ,CONSTRAINT pm_site_is_primary_key UNIQUE (is_primary)
);

CREATE TABLE pm_route (
    site_id UUID NOT NULL
    ,route_id UUID PRIMARY KEY NOT NULL
    ,parent_route_id UUID
    ,basename TEXT
    ,path TEXT
    ,template TEXT
    ,data JSON
    ,handler TEXT

    ,CONSTRAINT pm_route_site_id_path_key UNIQUE (site_id, path)
    ,CONSTRAINT pm_route_site_id_fkey FOREIGN KEY (site_id) REFERENCES pm_site (site_id)
    ,CONSTRAINT pm_route_parent_route_id_fkey FOREIGN KEY (parent_route_id) REFERENCES pm_route (route_id)
);

CREATE INDEX pm_route_site_id_idx ON pm_route (site_id);

CREATE INDEX pm_route_parent_route_id_idx ON pm_route (parent_route_id);

CREATE INDEX pm_route_path_idx ON pm_route (path);

CREATE INDEX pm_route_handler_idx ON pm_route (handler);

CREATE TABLE pm_route_closure (
    route_id UUID NOT NULL
    ,ancestor_route_id UUID NOT NULL
    ,depth INT

    ,CONSTRAINT pm_route_closure_route_id_ancestor_route_id_pkey PRIMARY KEY (route_id, ancestor_route_id)
    ,CONSTRAINT pm_route_closure_route_id_fkey FOREIGN KEY (route_id) REFERENCES pm_route (route_id)
    ,CONSTRAINT pm_route_closure_ancestor_route_id_fkey FOREIGN KEY (ancestor_route_id) REFERENCES pm_route (route_id)
);

CREATE INDEX pm_route_closure_ancestor_route_id_idx ON pm_route_closure (ancestor_route_id);
