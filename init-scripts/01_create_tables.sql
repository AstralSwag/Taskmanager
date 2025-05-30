    CREATE TABLE states (
        id UUID PRIMARY KEY,
        name VARCHAR(255) NOT NULL,
        color VARCHAR(50)
    );

    CREATE TABLE projects (
        id UUID PRIMARY KEY,
        name VARCHAR(255) NOT NULL,
        description TEXT,
        identifier VARCHAR(50) NOT NULL
    );

    CREATE TABLE estimates (
        id UUID PRIMARY KEY,
        name VARCHAR(255) NOT NULL,
        description TEXT,
        type VARCHAR(50)
    );

    CREATE TABLE estimate_points (
        id UUID PRIMARY KEY,
        estimate_id UUID REFERENCES estimates(id),
        value VARCHAR(50)
    );

    CREATE TABLE issues (
        id UUID PRIMARY KEY,
        name VARCHAR(255) NOT NULL,
        description_stripped TEXT,
        priority VARCHAR(50),
        state_id UUID REFERENCES states(id),
        point INTEGER,
        project_id UUID REFERENCES projects(id),
        sequence_id INTEGER,
        created_at TIMESTAMP WITH TIME ZONE,
        deleted_at TIMESTAMP WITH TIME ZONE,
        estimate_point_id UUID REFERENCES estimate_points(id)
    );

    CREATE TABLE issue_assignees (
        issue_id UUID REFERENCES issues(id),
        assignee_id UUID,
        created_at TIMESTAMP WITH TIME ZONE,
        deleted_at TIMESTAMP WITH TIME ZONE,
        PRIMARY KEY (issue_id, assignee_id)
    );