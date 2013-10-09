from fabric.api import local, env

def production():
    env['remote'] = 'production'

# producduction is default target for now
production()

def deploy():
    """
    Deploy to remote

    """
    local(('ansible-playbook ansible/deploy.yml '
           '-i ansible/{0}_hosts').format(env['remote']))
