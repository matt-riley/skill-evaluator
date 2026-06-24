#!/bin/bash
echo -e "\033[1;35m✨ Starting skill-eval loop...\033[0m"
sleep 0.5
echo -e "\033[1;34m[1/3] 🏃 Running evals (baseline & with-skill)...\033[0m"
sleep 1
echo -e "  \033[32m✔\033[0m eval-1 (Baseline) completed in 2.4s"
sleep 0.3
echo -e "  \033[32m✔\033[0m eval-1 (With-skill) completed in 1.8s"
sleep 0.8
echo -e "  \033[32m✔\033[0m eval-2 (Baseline) completed in 3.1s"
sleep 0.4
echo -e "  \033[32m✔\033[0m eval-2 (With-skill) completed in 2.2s"
sleep 0.5
echo ""
echo -e "\033[1;36m[2/3] 🎓 LLM-Grading assertions...\033[0m"
sleep 1.2
echo -e "  \033[32mPASS\033[0m The output includes a bar chart image file"
sleep 0.2
echo -e "  \033[32mPASS\033[0m The chart shows exactly 3 months"
sleep 0.4
echo -e "  \033[31mFAIL\033[0m Both axes are labeled \033[90m(Baseline only)\033[0m"
sleep 0.5
echo ""
echo -e "\033[1;33m[3/3] 📊 Aggregating benchmarks...\033[0m"
sleep 1
echo -e "\033[1mIteration 2 Results:\033[0m"
echo -e "  Pass Rate: \033[1;32m87.5%\033[0m \033[1;32m(+12.5%)\033[0m"
echo -e "  Avg Time:  \033[1;34m2.0s\033[0m \033[1;32m(-0.7s)\033[0m"
echo -e "  Avg Token: \033[1;35m1,402\033[0m \033[1;32m(-240)\033[0m"
echo ""
echo -e "\033[1;32m✅ Loop complete! Check benchmark.json for details.\033[0m"
sleep 2
