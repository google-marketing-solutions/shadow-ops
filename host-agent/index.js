const express = require('express');
const Docker = require('dockerode');

const app = express();
const docker = new Docker({ socketPath: '/var/run/docker.sock' });

app.use(express.json());

// Health check
app.get('/health', (req, res) => {
    res.send('OK');
});

// List containers
app.get('/containers', async (req, res) => {
    try {
        const containers = await docker.listContainers({ all: true });
        res.json(containers);
    } catch (err) {
        console.error('Error listing containers:', err);
        res.status(500).json({ error: err.message });
    }
});

// Inspect container
app.get('/containers/:id/json', async (req, res) => {
    try {
        const container = docker.getContainer(req.params.id);
        const data = await container.inspect();
        res.json(data);
    } catch (err) {
        console.error('Error inspecting container:', err);
        res.status(500).json({ error: err.message });
    }
});

// Create and start a container using dockerode API
app.post('/containers', async (req, res) => {
    const { token, port, containerName, containerImage } = req.body;

    if (!port || !containerName || !containerImage) {
        return res.status(400).json({ error: 'Missing required parameters' });
    }

    console.log(`Creating container ${containerName} on port ${port}`);

    try {
        const workspaceId = containerName.replace('shadowops-', '');
        const container = await docker.createContainer({
            Image: containerImage,
            name: containerName,
            Env: [
                `AGENT_PORT=${port}`,
                `WORKSPACE_ID=${workspaceId}`,
                'BROWSER_HEADLESS=false',
            ],
            HostConfig: {
                NetworkMode: 'host',
                RestartPolicy: {
                    Name: 'unless-stopped',
                },
            },
        });

        await container.start();
        console.log(`Container started successfully, ID: ${container.id}`);
        res.json({ id: container.id });
    } catch (err) {
        console.error('Error creating/starting container:', err);
        res.status(500).json({ error: err.message });
    }
});

// Stop and remove a container
app.delete('/containers/:id', async (req, res) => {
    try {
        const container = docker.getContainer(req.params.id);

        console.log(`Stopping container: ${req.params.id}`);
        try {
            await container.stop();
        } catch (stopErr) {
            console.warn(`Warning: Failed to stop container (might be already stopped): ${stopErr.message}`);
        }

        console.log(`Removing container: ${req.params.id}`);
        await container.remove();

        res.json({ success: true });
    } catch (err) {
        console.error('Error stopping/removing container:', err);
        res.status(500).json({ error: err.message });
    }
});

const PORT = process.env.PORT || 8081;
app.listen(PORT, '0.0.0.0', () => {
    console.log(`Host Agent listening on port ${PORT}`);
});
