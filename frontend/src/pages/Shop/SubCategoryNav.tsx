import type { CategoryInfo } from "../../types/shop";
import { COLORS } from "./theme";

interface Props {
  category: CategoryInfo | null;
  activeSubId: number | null;
  onSelect: (subId: number | null) => void;
}

/**
 * Bandeau de sous-catégories sous la nav principale, affiché seulement quand
 * une catégorie ayant des sous-catégories est sélectionnée. Port de
 * react.store.HeaderSubCategories + HeaderSubCategoryButton (Haxe).
 */
export function SubCategoryNav({ category, activeSubId, onSelect }: Props) {
  if (!category || !category.subcategories || category.subcategories.length === 0) {
    return null;
  }

  return (
    <div
      className="hidden sm:block"
      style={{
        backgroundColor: COLORS.bg3,
        padding: 5,
        textAlign: "left",
      }}
    >
      <div style={{ maxWidth: 1240, margin: "auto", padding: "0 10px" }}>
        {category.subcategories.map((sub) => {
          const isActive = sub.id === activeSubId;
          return (
            <button
              key={sub.id}
              type="button"
              onClick={() => onSelect(isActive ? null : sub.id)}
              style={{
                fontSize: "0.75rem",
                margin: "5px 5px",
                padding: "5px 10px",
                backgroundColor: COLORS.white,
                borderRadius: 16,
                border: "none",
                color: isActive ? "#E56403" : COLORS.darkGrey,
                fontWeight: isActive ? 700 : 400,
                cursor: "pointer",
                transition: "background-color 0.2s",
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.backgroundColor = "#DCDCDC";
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.backgroundColor = COLORS.white;
              }}
            >
              {sub.name}
            </button>
          );
        })}
      </div>
    </div>
  );
}
