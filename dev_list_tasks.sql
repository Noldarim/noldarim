-- Quick SQL to find existing tasks for dev workflow
-- Run with: sqlite3 your_db_file.db < dev_list_tasks.sql

SELECT 
    t.id as task_id,
    t.title,
    t.project_id,
    p.name as project_name,
    p.repository_path,
    t.status,
    t.created_at
FROM tasks t 
JOIN projects p ON t.project_id = p.id 
ORDER BY t.created_at DESC 
LIMIT 5;