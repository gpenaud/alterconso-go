import { useQuery } from '@tanstack/react-query'
import { fetchAmap, type AmapVendor } from '../api/amap'
import { Layout } from '../components/Layout'
import { Card, CardHeader } from '../components/ui/Card'

export function AmapPage() {
  const { data, isLoading } = useQuery({
    queryKey: ['amap'],
    queryFn: fetchAmap,
  })

  return (
    <Layout title="Producteurs">
      <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
        <div className="md:col-span-3 space-y-3">
          {isLoading ? (
            <p className="text-sm text-gray-500">Chargement…</p>
          ) : !data || data.vendors.length === 0 ? (
            <p className="text-sm text-gray-500">
              Aucun producteur associé à ce groupe.
            </p>
          ) : (
            data.vendors.map((v) => <VendorCard key={v.id} vendor={v} />)
          )}
        </div>

        <aside className="space-y-4">
          {data?.group && (
            <Card>
              <div className="px-5 py-4">
                <h3 className="italic text-lg m-0 text-gray-800">{data.group.name}</h3>
                {data.isGroupManager && (
                  <a
                    href="/amapadmin"
                    className="inline-flex items-center gap-1 mt-3 px-3 py-1.5 rounded border border-gray-300 text-sm text-gray-700 hover:bg-gray-50"
                  >
                    <i className="icon-edit" aria-hidden="true" />
                    Modifier
                  </a>
                )}
              </div>
            </Card>
          )}

          {data?.contact && (
            <Card>
              <CardHeader title="Contact principal" />
              <div className="px-5 py-4 text-sm space-y-1">
                <p>
                  <i className="icon-user text-gray-400" aria-hidden="true" />{' '}
                  <b>{data.contact.firstName} {data.contact.lastName}</b>
                </p>
                {data.contact.email && (
                  <p>
                    <i className="icon-mail text-gray-400" aria-hidden="true" />{' '}
                    <a
                      href={`mailto:${data.contact.email}`}
                      className="text-ac-green-dark hover:underline"
                    >
                      {data.contact.email}
                    </a>
                  </p>
                )}
                {data.contact.phone && (
                  <p>
                    <i className="icon-phone text-gray-400" aria-hidden="true" />{' '}
                    {data.contact.phone}
                  </p>
                )}
              </div>
            </Card>
          )}
        </aside>
      </div>
    </Layout>
  )
}

function VendorCard({ vendor }: { vendor: AmapVendor }) {
  return (
    <Card>
      <div className="p-4 flex flex-col md:flex-row gap-5">
        {/* Avatar + nom + ville */}
        <div className="md:w-32 flex-shrink-0 text-center">
          <div className="w-24 h-24 mx-auto bg-amber-50 border border-amber-200 rounded flex items-center justify-center">
            <i className="icon-farmer text-4xl text-amber-700" aria-hidden="true" />
          </div>
          <div className="font-bold text-sm mt-2">{vendor.name}</div>
          {vendor.city && (
            <div className="text-xs text-gray-500">
              {vendor.city}
              {vendor.zipCode && ` (${vendor.zipCode})`}
            </div>
          )}
        </div>

        {/* Catalogues + vignettes */}
        <div className="flex-1 min-w-0 space-y-4">
          {vendor.catalogs.map((cat) => (
            <div key={cat.id}>
              <a
                href={`/contract/view/${cat.id}`}
                className="italic font-bold text-ac-green-dark hover:underline"
              >
                {cat.name}
              </a>
              {cat.productImages.length > 0 && (
                <div className="flex flex-wrap gap-1.5 mt-2">
                  {cat.productImages.map((img, i) => (
                    <div
                      key={i}
                      title={img.name}
                      className="w-16 h-16 rounded border border-gray-200 bg-amber-50 overflow-hidden flex-shrink-0"
                    >
                      <img src={img.url} alt="" className="w-full h-full object-cover" />
                    </div>
                  ))}
                </div>
              )}
            </div>
          ))}
        </div>

        {/* Coordinateurs */}
        <div className="md:w-52 flex-shrink-0 text-sm space-y-3">
          {vendor.catalogs
            .filter((c) => c.coordinator)
            .map((cat) => (
              <div key={cat.id}>
                <b>Coordinateur :</b>
                <p className="mt-1 mb-0.5">
                  <i className="icon-user text-gray-400" aria-hidden="true" />{' '}
                  {cat.coordinator!.firstName} {cat.coordinator!.lastName}
                </p>
                {cat.coordinator!.email && (
                  <p className="m-0">
                    <i className="icon-mail text-gray-400" aria-hidden="true" />{' '}
                    <a
                      href={`mailto:${cat.coordinator!.email}`}
                      className="text-ac-green-dark hover:underline"
                    >
                      {cat.coordinator!.email}
                    </a>
                  </p>
                )}
                {cat.coordinator!.phone && (
                  <p className="m-0">
                    <i className="icon-phone text-gray-400" aria-hidden="true" />{' '}
                    {cat.coordinator!.phone}
                  </p>
                )}
              </div>
            ))}
        </div>
      </div>
    </Card>
  )
}
