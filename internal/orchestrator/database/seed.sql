-- Seed data for noldarim database
-- Updated for new schema with environment fields (worktree_path, container_id, branch_name, has_worktree, has_container)

-- Insert Projects
INSERT INTO projects (id, name, description, agent_id, last_updated_at, created_at) VALUES
('project_1', 'Authentication & Security', 'User authentication system with JWT tokens and secure middleware', 'agent_auth', '2024-01-15 14:30:00', '2024-01-10 09:00:00'),
('project_2', 'Frontend Development', 'React-based frontend with component library and modern UI', 'agent_frontend', '2024-01-14 16:45:00', '2024-01-12 10:30:00'),
('project_3', 'DevOps & CI/CD', 'Continuous integration and deployment pipeline setup', 'agent_devops', '2024-01-13 11:20:00', '2024-01-11 14:15:00');

-- Insert Tasks for Project 1 (Authentication & Security)
INSERT INTO tasks (id, title, description, status, project_id, branch, exec_history, agent_id, last_updated_at, created_at, worktree_path, container_id, branch_name, has_worktree, has_container) VALUES
('task_1', 'User Authentication Setup', 'Implement JWT-based authentication with bcrypt password hashing', 1, 'project_1', 'feature/user-authentication', 
'["git checkout -b feature/user-authentication", "npm install bcrypt jsonwebtoken", "touch auth.js middleware.js", "git add .", "git commit -m \"Add user authentication setup\""]', 
'agent_auth', '2024-01-15 14:30:00', '2024-01-10 09:15:00', '../noldarim-auth-task', 'auth_container_1', 'feature/user-authentication', 1, 1),

('task_2', 'Database Integration', 'Set up MongoDB connection with Mongoose ODM and user/product models', 0, 'project_1', 'feature/database-integration', 
'["git checkout -b feature/database-integration", "npm install mongoose", "mkdir models", "touch models/User.js models/Product.js"]', 
'agent_auth', '2024-01-15 12:45:00', '2024-01-10 11:30:00', '', '', 'feature/database-integration', 0, 0),

('task_3', 'API Endpoints', 'Create RESTful API routes for users and products with proper controllers', 2, 'project_1', 'feature/api-endpoints', 
'["git checkout -b feature/api-endpoints", "mkdir routes controllers", "touch routes/users.js routes/products.js", "git add .", "git commit -m \"Create API route structure\""]', 
'agent_auth', '2024-01-15 10:20:00', '2024-01-10 13:45:00', '../noldarim-api-task', 'api_container_1', 'feature/api-endpoints', 1, 1);

-- Insert Tasks for Project 2 (Frontend Development)
INSERT INTO tasks (id, title, description, status, project_id, branch, exec_history, agent_id, last_updated_at, created_at, worktree_path, container_id, branch_name, has_worktree, has_container) VALUES
('task_4', 'React App Initialization', 'Bootstrap React application with modern tooling and dependencies', 2, 'project_2', 'feature/frontend-setup', 
'["npx create-react-app frontend", "cd frontend", "npm install axios styled-components", "git add .", "git commit -m \"Initialize React frontend\""]', 
'agent_frontend', '2024-01-14 16:45:00', '2024-01-12 10:30:00', '../noldarim-frontend-task', 'frontend_container_1', 'feature/frontend-setup', 1, 1),

('task_5', 'Component Library', 'Build reusable UI components with PropTypes validation and styling', 1, 'project_2', 'feature/component-library', 
'["git checkout -b feature/component-library", "mkdir src/components src/hooks", "touch src/components/Button.js src/components/Modal.js", "npm install prop-types"]', 
'agent_frontend', '2024-01-14 15:20:00', '2024-01-12 14:15:00', '../noldarim-components-task', '', 'feature/component-library', 1, 0),

('task_6', 'State Management', 'Implement Redux for global state management with proper actions and reducers', 0, 'project_2', 'feature/state-management', 
'["git checkout -b feature/state-management", "npm install redux react-redux", "mkdir src/store src/actions src/reducers"]', 
'agent_frontend', '2024-01-14 13:10:00', '2024-01-12 16:20:00', '', '', 'feature/state-management', 0, 0);

-- Insert Tasks for Project 3 (DevOps & CI/CD)
INSERT INTO tasks (id, title, description, status, project_id, branch, exec_history, agent_id, last_updated_at, created_at, worktree_path, container_id, branch_name, has_worktree, has_container) VALUES
('task_7', 'CI/CD Pipeline Setup', 'Configure GitHub Actions for automated testing and deployment workflows', 0, 'project_3', 'feature/ci-cd-setup', 
'["mkdir .github/workflows", "touch .github/workflows/ci.yml", "touch .github/workflows/deploy.yml", "git add .", "git commit -m \"Add CI/CD pipeline configuration\""]', 
'agent_devops', '2024-01-13 11:20:00', '2024-01-11 14:15:00', '', '', 'feature/ci-cd-setup', 0, 0),

('task_8', 'Docker Configuration', 'Set up Docker containers for development and production environments', 1, 'project_3', 'feature/docker-setup', 
'["git checkout -b feature/docker-setup", "touch Dockerfile docker-compose.yml", "mkdir .docker", "touch .docker/nginx.conf"]', 
'agent_devops', '2024-01-13 10:45:00', '2024-01-11 15:30:00', '../noldarim-docker-task', 'docker_container_1', 'feature/docker-setup', 1, 1),

('task_9', 'Monitoring & Logging', 'Implement application monitoring with logging and metrics collection', 2, 'project_3', 'feature/monitoring', 
'["git checkout -b feature/monitoring", "npm install winston prometheus-client", "mkdir monitoring", "touch monitoring/logger.js monitoring/metrics.js", "git add .", "git commit -m \"Add monitoring setup\""]', 
'agent_devops', '2024-01-13 09:15:00', '2024-01-11 16:45:00', '../noldarim-monitoring-task', 'monitoring_container_1', 'feature/monitoring', 1, 1);