import { useEffect, useMemo, useState } from "react";
import { useParams, useSearchParams } from "react-router-dom";
import { useShopData } from "../hooks/useShop";
import { useCartStore } from "../store/cart";
import { parseDateTime } from "../utils/format";
import type { ProductInfo, VendorInfo } from "../types/shop";
import { Header } from "./Shop/Header";
import { CategoryNav } from "./Shop/CategoryNav";
import { SubCategoryNav } from "./Shop/SubCategoryNav";
import { CategorySection } from "./Shop/CategorySection";
import { ProductModal } from "./Shop/ProductModal";
import { CartPanel } from "./Shop/CartPanel";
import { ProductFilters, type TagFilter } from "./Shop/ProductFilters";

export function ShopPage() {
  const params = useParams<{ multiDistribId: string }>();
  const multiDistribId = Number(params.multiDistribId);
  // ?userId=N : un admin commande pour le compte d'un membre. L'API /orders
  // accepte ce userId si l'appelant est gestionnaire du groupe.
  const [searchParams] = useSearchParams();
  const targetUserId = useMemo(() => {
    const v = searchParams.get("userId");
    const n = v ? Number(v) : NaN;
    return Number.isFinite(n) && n > 0 ? n : undefined;
  }, [searchParams]);

  const { isLoading, error, init, categories, catalog } = useShopData(multiDistribId);
  const setMd = useCartStore((s) => s.setMultiDistrib);
  const [activeCategory, setActiveCategory] = useState<number | null>(null);
  const [activeSubcategory, setActiveSubcategory] = useState<number | null>(null);
  const [search, setSearch] = useState("");
  const [tagFilters, setTagFilters] = useState<Set<TagFilter>>(new Set());
  const [isSticky, setIsSticky] = useState(false);

  // Détecte le passage en mode sticky du bandeau d'en-tête. Utilise un seuil
  // simple basé sur scrollY plutôt qu'une lib (legacy : sticky-events).
  useEffect(() => {
    const onScroll = () => setIsSticky(window.scrollY > 80);
    window.addEventListener("scroll", onScroll, { passive: true });
    onScroll();
    return () => window.removeEventListener("scroll", onScroll);
  }, []);
  const [openProduct, setOpenProduct] = useState<{
    product: ProductInfo;
    vendor?: VendorInfo;
  } | null>(null);
  const [cartOpen, setCartOpen] = useState(false);

  const toggleTag = (tag: TagFilter) =>
    setTagFilters((prev) => {
      const next = new Set(prev);
      if (next.has(tag)) next.delete(tag);
      else next.add(tag);
      return next;
    });

  const activeCategoryInfo = useMemo(
    () =>
      activeCategory != null
        ? categories.find((c) => c.id === activeCategory) ?? null
        : null,
    [categories, activeCategory],
  );

  const handleSelectCategory = (id: number | null) => {
    setActiveCategory(id);
    setActiveSubcategory(null);
  };

  useEffect(() => {
    if (!Number.isNaN(multiDistribId)) {
      setMd(multiDistribId);
    }
  }, [multiDistribId, setMd]);

  const startDate = useMemo(
    () => (init ? parseDateTime(init.distributionStartDate) : null),
    [init],
  );
  const endDate = useMemo(
    () => (init ? parseDateTime(init.distributionEndDate) : null),
    [init],
  );

  const visibleCategories = useMemo(() => {
    if (!catalog) return [];
    let cats = catalog.categories;
    if (activeCategory != null) cats = cats.filter((c) => c.info.id === activeCategory);
    if (activeSubcategory != null) {
      cats = cats
        .map((c) => ({
          ...c,
          subcategories: c.subcategories.filter((sc) => sc.info.id === activeSubcategory),
        }))
        .filter((c) => c.subcategories.length > 0);
    }
    if (search || tagFilters.size > 0) {
      const q = search.toLowerCase();
      cats = cats
        .map((c) => ({
          ...c,
          subcategories: c.subcategories
            .map((sc) => ({
              ...sc,
              products: sc.products.filter((p) => {
                if (q && !p.name.toLowerCase().includes(q)) return false;
                if (tagFilters.has("organic") && !p.organic) return false;
                if (tagFilters.has("bulk") && !p.bulk) return false;
                return true;
              }),
            }))
            .filter((sc) => sc.products.length > 0),
        }))
        .filter((c) => c.subcategories.length > 0);
    }
    return cats;
  }, [catalog, activeCategory, activeSubcategory, search, tagFilters]);

  if (!Number.isInteger(multiDistribId) || multiDistribId <= 0) {
    return (
      <div className="p-8 text-center text-red-600">
        Identifiant de distribution invalide.
      </div>
    );
  }
  if (isLoading) {
    return <div className="p-8 text-center text-gray-500">Chargement…</div>;
  }
  if (error) {
    return (
      <div className="p-8 text-center text-red-600">
        Erreur : {(error as Error).message}
      </div>
    );
  }
  if (!init || !catalog) return null;

  return (
    <div className="min-h-screen" style={{ backgroundColor: "#fff" }}>
      <div
        style={{
          position: "sticky",
          top: 0,
          zIndex: 30,
          backgroundColor: "#fff",
          boxShadow: isSticky ? "0 2px 8px rgba(0,0,0,0.08)" : "none",
          transition: "box-shadow 0.2s",
        }}
      >
        <Header
          startDate={startDate}
          endDate={endDate}
          place={init.place}
          search={search}
          onSearch={setSearch}
          onCartClick={() => setCartOpen(true)}
        />

        <CategoryNav
          categories={categories}
          activeId={activeCategory}
          onSelect={handleSelectCategory}
          compact={isSticky}
        />

        <SubCategoryNav
          category={activeCategoryInfo}
          activeSubId={activeSubcategory}
          onSelect={setActiveSubcategory}
        />
      </div>

      <ProductFilters active={tagFilters} onToggle={toggleTag} />

      <main style={{ maxWidth: 1240, margin: "auto", padding: "16px 16px 32px" }}>
        {visibleCategories.length === 0 && (
          <div className="text-center text-gray-500" style={{ padding: 40 }}>
            Aucun produit trouvé.
          </div>
        )}
        {visibleCategories.map((cat) => (
          <CategorySection
            key={cat.info.id}
            category={cat}
            vendors={init.vendors}
            onProductClick={(product, vendor) =>
              setOpenProduct({ product, vendor })
            }
          />
        ))}
      </main>

      {openProduct && openProduct.vendor && (
        <ProductModal
          product={openProduct.product}
          vendor={openProduct.vendor}
          onClose={() => setOpenProduct(null)}
        />
      )}

      {cartOpen && (
        <CartPanel targetUserId={targetUserId} onClose={() => setCartOpen(false)} />
      )}

      {/* FAB "remonter en haut" — apparaît une fois scrollé (legacy
           HeaderWrapper.renderFab quand isSticky). */}
      {isSticky && (
        <button
          type="button"
          aria-label="Remonter en haut"
          title="Remonter en haut"
          onClick={() => window.scrollTo({ top: 0, behavior: "smooth" })}
          className="flex items-center justify-center transition-shadow hover:shadow-lg"
          style={{
            position: "fixed",
            right: 24,
            bottom: 24,
            width: 48,
            height: 48,
            borderRadius: "50%",
            background: "#a53fa1",
            color: "#fff",
            border: "none",
            cursor: "pointer",
            zIndex: 40,
            boxShadow: "0 4px 12px rgba(0,0,0,0.20)",
            fontSize: 20,
          }}
        >
          <i className="icon-chevron-up" aria-hidden="true" />
        </button>
      )}
    </div>
  );
}
