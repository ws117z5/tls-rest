import React from "react";
import { imageUrl } from "./images";
import { FolderSummary } from "./folders";

interface FolderDeckProps {
    folder: FolderSummary;
    onClick: () => void;
}

// A fanned-out deck of the folder's first 3 images (front card = first image),
// with the total image count badged in the bottom-right corner.
const FolderDeck: React.FC<FolderDeckProps> = ({ folder, onClick }) => {
    const cards = folder.previews.slice(0, 3);
    const offsets = [
        { rotate: 0, translate: 0 },
        { rotate: 4, translate: 3 },
        { rotate: -6, translate: -6 },
    ];
    // Draw back-to-front so the first image ends up visually on top.
    const drawOrder = [...cards].map((ref, i) => ({ ref, offset: offsets[i] })).reverse();

    return (
        <div className="card h-100" style={{ cursor: "pointer" }} onClick={onClick} title={folder.folder}>
            <div style={{ position: "relative", aspectRatio: "1 / 1", background: "#f5f5f5" }}>
                {drawOrder.map(({ ref, offset }) => (
                    <img
                        key={ref.id}
                        src={imageUrl(ref, { preview: true })}
                        alt=""
                        style={{
                            position: "absolute",
                            inset: 8,
                            width: "calc(100% - 16px)",
                            height: "calc(100% - 16px)",
                            objectFit: "cover",
                            borderRadius: 6,
                            border: "2px solid #fff",
                            boxShadow: "0 2px 6px rgba(0,0,0,0.18)",
                            transform: `rotate(${offset.rotate}deg) translateX(${offset.translate}px)`,
                        }}
                    />
                ))}
                {cards.length === 0 && (
                    <div
                        style={{
                            position: "absolute",
                            inset: 0,
                            display: "flex",
                            alignItems: "center",
                            justifyContent: "center",
                            color: "#999",
                            fontSize: 12,
                        }}
                    />
                )}
                <span
                    style={{
                        position: "absolute",
                        right: 6,
                        bottom: 6,
                        background: "rgba(0,0,0,0.72)",
                        color: "#fff",
                        borderRadius: 999,
                        padding: "2px 8px",
                        fontSize: 12,
                        fontWeight: 600,
                        lineHeight: "16px",
                    }}
                >
                    {folder.count}
                </span>
            </div>
            <div className="card-body p-2">
                <div className="text-truncate small fw-semibold">{folder.folder}</div>
            </div>
        </div>
    );
};

export default FolderDeck;
