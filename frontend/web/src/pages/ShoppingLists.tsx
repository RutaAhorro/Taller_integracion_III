import { useEffect, useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useAuth } from '@/hooks/useAuth';
import { useToast } from '@/contexts/ToastContext';
import { ROUTES } from '@/core/routes';
import { listasApi, type Lista, type ListaDetalle, type EstadoLista } from '@/features/listas/listas-api';

const estados: Record<EstadoLista, string> = { activa: 'Activa', completada: 'Completada', archivada: 'Archivada' };
const mensajeError = (error: unknown) => error instanceof Error ? error.message : 'No se pudo conectar con el servicio de listas.';

export default function ShoppingLists() {
  const { user } = useAuth();
  return <ListsContent key={user?.id} />;
}

function ListsContent() {
  const { toast } = useToast();
  const [listas, setListas] = useState<Lista[]>([]);
  const [selected, setSelected] = useState<string | null>(null);
  const [nombre, setNombre] = useState('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [detailBusy, setDetailBusy] = useState(false);
  const [error, setError] = useState('');
  const [revision, setRevision] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError('');
    listasApi.listar(controller.signal).then(({ listas: data }) => {
      if (!controller.signal.aborted) setListas(data);
    }).catch((err: unknown) => {
      if (!controller.signal.aborted) setError(mensajeError(err));
    }).finally(() => {
      if (!controller.signal.aborted) setLoading(false);
    });
    return () => controller.abort();
  }, [revision]);

  async function crear(event: FormEvent) {
    event.preventDefault();
    if (saving || !nombre.trim()) return;
    setSaving(true);
    setError('');
    try {
      const lista = await listasApi.crear(nombre.trim());
      setListas(current => [lista, ...current]);
      setSelected(lista.id);
      setNombre('');
      toast('Lista creada y guardada', 'success');
    } catch (err) { setError(mensajeError(err)); }
    finally { setSaving(false); }
  }

  return (
    <main className="mx-auto w-full max-w-5xl space-y-6 p-4 md:p-8">
      <header>
        <h1 className="text-2xl font-bold">Mis listas de compra</h1>
        <p className="mt-2 text-muted-foreground">Gestiona tus listas guardadas en tu cuenta y consúltalas entre sesiones.</p>
      </header>
      <p className="rounded-xl border border-border bg-secondary/40 p-4 text-sm">
        Disponible: crear, consultar, editar y eliminar listas. Agregar o quitar productos y validar stock estará disponible en una próxima entrega (SUP-241). El carrito temporal del catálogo aún no se sincroniza con estas listas.
      </p>
      <form onSubmit={crear} className="space-y-2">
        <label htmlFor="new-list-name" className="text-sm font-medium">Nombre de la nueva lista</label>
        <div className="flex flex-col gap-2 sm:flex-row">
          <Input id="new-list-name" value={nombre} onChange={e => setNombre(e.target.value)} maxLength={150} required disabled={saving || loading || detailBusy} placeholder="Por ejemplo: compras de la semana" />
          <Button type="submit" disabled={saving || loading || detailBusy || !nombre.trim()}>{saving ? 'Creando…' : 'Crear lista'}</Button>
        </div>
      </form>
      {error && <div role="alert" className="space-y-2 text-destructive"><p>{error}</p><Link to={ROUTES.LOGIN} className="underline">Ir a iniciar sesión</Link></div>}
      <Button variant="outline" disabled={loading || saving || detailBusy} onClick={() => setRevision(value => value + 1)}>Actualizar listas</Button>
      {loading ? <p role="status">Cargando listas…</p> : (
        <div className="grid gap-6 md:grid-cols-2">
          <section aria-label="Listas guardadas">
            {listas.length === 0 ? <p>{error ? 'No se pudieron cargar las listas.' : 'Aún no tienes listas. Crea la primera para empezar.'}</p> : (
              <ul className="space-y-3">
                {listas.map(lista => <li key={lista.id}>
                  <button disabled={saving || detailBusy} onClick={() => setSelected(lista.id)} aria-pressed={selected === lista.id} className={`w-full rounded-xl border p-4 text-left disabled:opacity-60 ${selected === lista.id ? 'border-primary bg-primary/5' : 'border-border bg-card'}`}>
                    <span className="block break-words font-semibold">{lista.nombre}</span>
                    <span className="text-sm text-muted-foreground">{estados[lista.estado]} · {lista.total_articulos} artículos</span>
                  </button>
                </li>)}
              </ul>
            )}
          </section>
          {selected ? <fieldset disabled={saving} className="min-w-0"><ListDetail key={selected} id={selected}
            onBusy={setDetailBusy}
            onUpdated={lista => setListas(current => current.map(item => item.id === lista.id ? lista : item))}
            onDeleted={() => { setListas(current => current.filter(item => item.id !== selected)); setSelected(null); }}
          /></fieldset> : <p className="text-muted-foreground">Selecciona una lista para consultar su detalle.</p>}
        </div>
      )}
    </main>
  );
}

function ListDetail({ id, onUpdated, onDeleted, onBusy }: { id: string; onUpdated: (lista: Lista) => void; onDeleted: () => void; onBusy: (busy: boolean) => void }) {
  const { toast } = useToast();
  const [lista, setLista] = useState<ListaDetalle | null>(null);
  const [nombre, setNombre] = useState('');
  const [estado, setEstado] = useState<EstadoLista>('activa');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [revision, setRevision] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError('');
    listasApi.obtener(id, controller.signal).then(data => {
      if (controller.signal.aborted) return;
      setLista(data); setNombre(data.nombre); setEstado(data.estado);
    }).catch((err: unknown) => {
      if (!controller.signal.aborted) setError(mensajeError(err));
    }).finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [id, revision]);

  async function guardar(event: FormEvent) {
    event.preventDefault();
    if (saving || !nombre.trim()) return;
    setSaving(true); onBusy(true); setError('');
    try {
      const updated = await listasApi.actualizar(id, nombre.trim(), estado);
      setLista(current => current ? { ...current, ...updated } : current);
      setNombre(updated.nombre); setEstado(updated.estado);
      onUpdated(updated);
      toast('Lista actualizada', 'success');
    } catch (err) { setError(mensajeError(err)); }
    finally { setSaving(false); onBusy(false); }
  }

  async function eliminar() {
    if (saving || !window.confirm(`¿Eliminar la lista “${lista?.nombre}” y todos sus artículos?`)) return;
    setSaving(true); onBusy(true); setError('');
    try {
      await listasApi.eliminar(id);
      onDeleted();
      toast('Lista eliminada', 'success');
    } catch (err) { setError(mensajeError(err)); }
    finally { setSaving(false); onBusy(false); }
  }

  return <section aria-label="Detalle de la lista" className="min-w-0 space-y-4 rounded-xl border border-border bg-card p-4">
    <h2 className="text-lg font-semibold">Detalle de la lista</h2>
    {error && <p role="alert" className="text-destructive">{error}</p>}
    {loading ? <p role="status">Cargando detalle…</p> : lista ? <>
      <form onSubmit={guardar} className="space-y-3">
        <label htmlFor="list-name" className="block text-sm font-medium">Nombre</label>
        <Input id="list-name" value={nombre} onChange={e => setNombre(e.target.value)} maxLength={150} required disabled={saving} />
        <label htmlFor="list-state" className="block text-sm font-medium">Estado</label>
        <select id="list-state" value={estado} onChange={e => setEstado(e.target.value as EstadoLista)} disabled={saving} className="w-full rounded-md border border-input bg-background p-2">
          {Object.entries(estados).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
        </select>
        <div className="flex flex-wrap gap-2">
          <Button type="submit" disabled={saving || !nombre.trim()}>{saving ? 'Procesando…' : 'Guardar cambios'}</Button>
          <Button type="button" variant="destructive" disabled={saving} onClick={eliminar}>Eliminar lista</Button>
        </div>
      </form>
      <h3 className="font-semibold">Artículos (solo lectura)</h3>
      {lista.articulos.length === 0 ? <p className="text-sm text-muted-foreground">Esta lista aún no tiene artículos.</p> : <ul className="space-y-2 text-sm">
        {lista.articulos.map(item => <li key={item.id} className="break-words rounded-md border border-border p-2">
          <p>Producto: {item.producto_normalizado_id}</p>
          <p>Cantidad: {item.cantidad} · {item.esta_comprado ? 'Comprado' : 'Pendiente'}</p>
        </li>)}
      </ul>}
    </> : <Button variant="outline" onClick={() => setRevision(value => value + 1)}>Reintentar detalle</Button>}
  </section>;
}
