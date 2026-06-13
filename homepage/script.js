// scroll reveal system
const elements = document.querySelectorAll(".reveal");

const observer = new IntersectionObserver(entries => {
    entries.forEach(entry => {
        if(entry.isIntersecting){
            entry.target.classList.add("visible");
        }
    });
}, {
    threshold: 0.15
});

elements.forEach(el => observer.observe(el));

// smooth anchor scrolling (clean UX, not 2006 browser vibes)
document.querySelectorAll('a[href^="#"]').forEach(a => {
    a.addEventListener("click", e => {
        e.preventDefault();
        const target = document.querySelector(a.getAttribute("href"));
        if(target){
            target.scrollIntoView({ behavior: "smooth" });
        }
    });
});
