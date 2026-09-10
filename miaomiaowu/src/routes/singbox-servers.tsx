// sing-box 服务器管理 —— 通过 SSH 管理远端 sing-box 内核节点。
//
// 功能：服务器 CRUD、SSH 连接测试、状态查询、配置部署、服务重启。
// 所有管理员端点走 /api/admin/singbox-servers。
import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, redirect } from '@tanstack/react-router'
import {
  Loader2,
  Plus,
  Server,
  Trash2,
  Pencil,
  Wifi,
  Activity,
  Rocket,
  Power,
  RefreshCw,
} from 'lucide-react'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import { profileQueryFn } from '@/lib/profile'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { ConfirmDialog } from '@/components/confirm-dialog'

export const Route = createFileRoute('/singbox-servers')({
  beforeLoad: async ({ context }) => {
    let profile: { is_admin?: boolean } | undefined
    try {
      profile = await (
        context as {
          queryClient: {
            fetchQuery: (o: unknown) => Promise<{ is_admin?: boolean }>
          }
        }
      ).queryClient.fetchQuery({
        queryKey: ['profile'],
        queryFn: profileQueryFn,
        staleTime: 5 * 60 * 1000,
      })
    } catch {
      throw redirect({ to: '/login' })
    }
    if (!profile?.is_admin) {
      throw redirect({ to: '/' })
    }
  },
  component: SingboxServersPage,
})

interface SingboxServer {
  id: number
  name: string
  host: string
  port: number
  ssh_user: string
  auth_type: string
  has_auth: boolean
  config_path: string
  api_port: number
  singbox_port: number
  singbox_user: string
  enabled: boolean
  has_status: string
  created_at: string
  updated_at: string
}

interface ServerForm {
  name: string
  host: string
  port: number
  ssh_user: string
  auth_type: string
  auth_data: string
  config_path: string
  api_port: number
  singbox_port: number
  singbox_user: string
  enabled: boolean
}

const EMPTY_FORM: ServerForm = {
  name: '',
  host: '',
  port: 22,
  ssh_user: 'root',
  auth_type: 'password',
  auth_data: '',
  config_path: '/etc/sing-box/config.json',
  api_port: 9090,
  singbox_port: 7890,
  singbox_user: '',
  enabled: true,
}

function SingboxServersPage() {
  const queryClient = useQueryClient()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [form, setForm] = useState<ServerForm>(EMPTY_FORM)
  const [deleteTarget, setDeleteTarget] = useState<SingboxServer | null>(null)

  const { data: servers = [], isLoading } = useQuery({
    queryKey: ['singbox-servers'],
    queryFn: async () => {
      const res = await api.get('/api/admin/singbox-servers')
      return res.data as SingboxServer[]
    },
  })

  const createMutation = useMutation({
    mutationFn: async (data: ServerForm) => {
      await api.post('/api/admin/singbox-servers', data)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['singbox-servers'] })
      toast.success('服务器已创建')
      setDialogOpen(false)
    },
    onError: (err: any) => toast.error(err?.response?.data?.error || '创建失败'),
  })

  const updateMutation = useMutation({
    mutationFn: async ({ id, data }: { id: number; data: ServerForm }) => {
      await api.put(`/api/admin/singbox-servers/${id}`, data)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['singbox-servers'] })
      toast.success('服务器已更新')
      setDialogOpen(false)
    },
    onError: (err: any) => toast.error(err?.response?.data?.error || '更新失败'),
  })

  const deleteMutation = useMutation({
    mutationFn: async (id: number) => {
      await api.delete(`/api/admin/singbox-servers/${id}`)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['singbox-servers'] })
      toast.success('服务器已删除')
      setDeleteTarget(null)
    },
    onError: (err: any) => toast.error(err?.response?.data?.error || '删除失败'),
  })

  const actionMutation = useMutation({
    mutationFn: async ({ id, action }: { id: number; action: string }) => {
      const res = await api.post(`/api/admin/singbox-servers/${id}/${action}`)
      return res.data
    },
    onSuccess: (_data, vars) => {
      queryClient.invalidateQueries({ queryKey: ['singbox-servers'] })
      const labels: Record<string, string> = {
        test: '连接测试',
        status: '状态查询',
        deploy: '部署',
        restart: '重启',
        'sync-node': '同步节点',
      }
      toast.success(`${labels[vars.action] || vars.action}成功`)
    },
    onError: (err: any, vars) => {
      const labels: Record<string, string> = {
        test: '连接测试',
        status: '状态查询',
        deploy: '部署',
        restart: '重启',
        'sync-node': '同步节点',
      }
      toast.error(`${labels[vars.action] || vars.action}失败: ${err?.response?.data?.error || err.message}`)
    },
  })

  function openCreate() {
    setForm(EMPTY_FORM)
    setEditingId(null)
    setDialogOpen(true)
  }

  function openEdit(srv: SingboxServer) {
    setForm({
      name: srv.name,
      host: srv.host,
      port: srv.port,
      ssh_user: srv.ssh_user,
      auth_type: srv.auth_type || 'password',
      auth_data: '',
      config_path: srv.config_path,
      api_port: srv.api_port,
      singbox_port: srv.singbox_port,
      singbox_user: srv.singbox_user,
      enabled: srv.enabled,
    })
    setEditingId(srv.id)
    setDialogOpen(true)
  }

  function handleSubmit() {
    if (!form.name || !form.host) {
      toast.error('名称和主机不能为空')
      return
    }
    if (editingId) {
      updateMutation.mutate({ id: editingId, data: form })
    } else {
      createMutation.mutate(form)
    }
  }

  function handleAction(id: number, action: string) {
    actionMutation.mutate({ id, action })
  }

  const busy = createMutation.isPending || updateMutation.isPending

  return (
    <div className="container mx-auto p-4 space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Server className="h-6 w-6" />
          <h1 className="text-2xl font-bold">sing-box 服务器</h1>
        </div>
        <Button onClick={openCreate}>
          <Plus className="mr-1 h-4 w-4" />
          添加服务器
        </Button>
      </div>

      {isLoading ? (
        <div className="flex justify-center py-8">
          <Loader2 className="h-6 w-6 animate-spin" />
        </div>
      ) : servers.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12 text-muted-foreground">
            <Server className="mb-2 h-12 w-12 opacity-30" />
            <p>暂无 sing-box 服务器，点击「添加服务器」开始</p>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-3">
          {servers.map((srv) => (
            <Card key={srv.id}>
              <CardContent className="p-4 flex items-center justify-between gap-4">
                <div className="flex items-center gap-3 min-w-0 flex-1">
                  <div className="flex flex-col gap-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="font-semibold truncate">{srv.name}</span>
                      <Badge variant={srv.enabled ? 'default' : 'secondary'}>
                        {srv.enabled ? '启用' : '停用'}
                      </Badge>
                      {srv.has_status && (
                        <Badge variant="outline" className="text-xs">
                          {srv.has_status}
                        </Badge>
                      )}
                    </div>
                    <div className="text-sm text-muted-foreground truncate">
                      {srv.host}:{srv.port} | ssh: {srv.ssh_user} | 入站: {srv.singbox_port} | API: {srv.api_port}
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-1 shrink-0">
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => handleAction(srv.id, 'test')}
                    disabled={actionMutation.isPending}
                    title="连接测试"
                  >
                    <Wifi className="h-4 w-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => handleAction(srv.id, 'status')}
                    disabled={actionMutation.isPending}
                    title="状态查询"
                  >
                    <Activity className="h-4 w-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => handleAction(srv.id, 'deploy')}
                    disabled={actionMutation.isPending}
                    title="部署配置"
                  >
                    <Rocket className="h-4 w-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => handleAction(srv.id, 'restart')}
                    disabled={actionMutation.isPending}
                    title="重启服务"
                  >
                    <Power className="h-4 w-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => handleAction(srv.id, 'sync-node')}
                    disabled={actionMutation.isPending}
                    title="同步节点"
                  >
                    <RefreshCw className="h-4 w-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => openEdit(srv)}
                    title="编辑"
                  >
                    <Pencil className="h-4 w-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => setDeleteTarget(srv)}
                    title="删除"
                  >
                    <Trash2 className="h-4 w-4 text-destructive" />
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>{editingId ? '编辑服务器' : '添加 sing-box 服务器'}</DialogTitle>
            <DialogDescription>
              {editingId ? '修改服务器信息。认证字段留空表示不修改。' : '填写远端服务器的 SSH 连接信息。'}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <div className="grid grid-cols-2 gap-3">
              <div>
                <Label>名称</Label>
                <Input
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  placeholder="my-server"
                />
              </div>
              <div>
                <Label>主机</Label>
                <Input
                  value={form.host}
                  onChange={(e) => setForm({ ...form, host: e.target.value })}
                  placeholder="1.2.3.4"
                />
              </div>
            </div>
            <div className="grid grid-cols-3 gap-3">
              <div>
                <Label>SSH 端口</Label>
                <Input
                  type="number"
                  value={form.port}
                  onChange={(e) => setForm({ ...form, port: +e.target.value })}
                />
              </div>
              <div>
                <Label>SSH 用户</Label>
                <Input
                  value={form.ssh_user}
                  onChange={(e) => setForm({ ...form, ssh_user: e.target.value })}
                />
              </div>
              <div>
                <Label>认证方式</Label>
                <Select
                  value={form.auth_type}
                  onValueChange={(v) => setForm({ ...form, auth_type: v })}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="password">密码</SelectItem>
                    <SelectItem value="private_key">私钥</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            <div>
              <Label>{form.auth_type === 'password' ? '密码' : '私钥内容'}</Label>
              {form.auth_type === 'password' ? (
                <Input
                  type="password"
                  value={form.auth_data}
                  onChange={(e) => setForm({ ...form, auth_data: e.target.value })}
                  placeholder={editingId ? '留空不修改' : '输入密码'}
                />
              ) : (
                <textarea
                  className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm font-mono"
                  rows={3}
                  value={form.auth_data}
                  onChange={(e) => setForm({ ...form, auth_data: e.target.value })}
                  placeholder={editingId ? '留空不修改' : '粘贴 PEM 私钥'}
                />
              )}
            </div>
            <div className="grid grid-cols-3 gap-3">
              <div>
                <Label>入站端口</Label>
                <Input
                  type="number"
                  value={form.singbox_port}
                  onChange={(e) => setForm({ ...form, singbox_port: +e.target.value })}
                />
              </div>
              <div>
                <Label>API 端口</Label>
                <Input
                  type="number"
                  value={form.api_port}
                  onChange={(e) => setForm({ ...form, api_port: +e.target.value })}
                />
              </div>
              <div>
                <Label>配置路径</Label>
                <Input
                  value={form.config_path}
                  onChange={(e) => setForm({ ...form, config_path: e.target.value })}
                />
              </div>
            </div>
            <label className="flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                checked={form.enabled}
                onChange={(e) => setForm({ ...form, enabled: e.target.checked })}
                className="rounded"
              />
              <span className="text-sm">启用</span>
            </label>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>
              取消
            </Button>
            <Button onClick={handleSubmit} disabled={busy}>
              {busy && <Loader2 className="mr-1 h-4 w-4 animate-spin" />}
              {editingId ? '保存' : '创建'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(v) => !v && setDeleteTarget(null)}
        title="删除服务器"
        desc={`确定删除「${deleteTarget?.name}」吗？此操作不可恢复。`}
        handleConfirm={() => deleteTarget && deleteMutation.mutate(deleteTarget.id)}
        destructive
      />
    </div>
  )
}